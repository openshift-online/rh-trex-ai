package audit

import (
	"context"
	"log/slog"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/authz"
)

// UnaryBegin is the outer gRPC hook. TRex places it before authentication.
// Methods matching a bypass prefix are not audited.
func (s *Service) UnaryBegin(bypass []string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		if !s.Enabled() || bypassed(info.FullMethod, bypass) {
			return handler(ctx, req)
		}
		ctx, obs := authz.WithObservation(ctx)
		ctx, st := withState(ctx)
		panicked := true
		defer func() {
			if panicked {
				s.finishGRPC(ctx, info.FullMethod, obs, st, codes.Internal, "panic", false)
			}
		}()
		resp, err = handler(ctx, req)
		panicked = false
		s.finishGRPC(ctx, info.FullMethod, obs, st, grpcCode(err), "", false)
		return resp, err
	}
}

// UnaryRecord is the inner gRPC hook. TRex places it right after
// authorization. For an allowed call that is recorded, it writes the success
// event in the call's transaction before the handler runs, and returns
// codes.Unavailable when that write fails.
func (s *Service) UnaryRecord() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !s.Enabled() {
			return handler(ctx, req)
		}
		obs := authz.ObservationFromContext(ctx)
		st := stateFrom(ctx)
		if obs == nil || obs.Request == nil || st == nil || !s.recordsAllowed(obs.Request.Action, obs.Request.Resource.Kind) {
			return handler(ctx, req)
		}
		e := grpcEvent(ctx, info.FullMethod, obs.Request, OutcomeSuccess, "", codes.OK)
		e.Actor = actorFromSubject(authz.SubjectFromContext(ctx))
		if err := s.Record(ctx, e); err != nil {
			st.insertFailed = true
			slog.ErrorContext(ctx, "Unable to write audit event; refusing call", "error", err)
			return nil, status.Error(codes.Unavailable, "audit unavailable")
		}
		st.inserted = true
		return handler(ctx, req)
	}
}

// StreamBegin is the outer gRPC hook for streams. Streams hold no
// transaction, so every outcome is written when the stream ends.
func (s *Service) StreamBegin(bypass []string) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !s.Enabled() || bypassed(info.FullMethod, bypass) {
			return handler(srv, ss)
		}
		ctx, obs := authz.WithObservation(ss.Context())
		ctx, st := withState(ctx)
		panicked := true
		defer func() {
			if panicked {
				s.finishGRPC(ctx, info.FullMethod, obs, st, codes.Internal, "panic", true)
			}
		}()
		err := handler(srv, &contextStream{ServerStream: ss, ctx: ctx})
		panicked = false
		s.finishGRPC(ctx, info.FullMethod, obs, st, grpcCode(err), "", true)
		return err
	}
}

type contextStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (c *contextStream) Context() context.Context { return c.ctx }

func (s *Service) finishGRPC(ctx context.Context, method string, obs *authz.Observation, st *requestState, code codes.Code, reason string, stream bool) {
	req := obs.Request
	switch {
	case obs.Undeclared:
		e := grpcEvent(ctx, method, req, OutcomeDenied, "method has no declared resource", code)
		if req == nil {
			e.Action = "unknown"
		}
		s.recordDetached(ctx, e)
	case obs.Err != nil:
		s.recordDetached(ctx, grpcEvent(ctx, method, req, OutcomeError, "authorization unavailable", code))
	case obs.Decided && !obs.Decision.Allowed:
		s.recordDetached(ctx, grpcEvent(ctx, method, req, OutcomeDenied, denyReason(obs.Decision), code))
	case req == nil:
		if code == codes.Unauthenticated {
			e := grpcEvent(ctx, method, nil, OutcomeUnauthenticated, "authentication failed", code)
			e.Action = "unknown"
			s.streamOnly(ctx, e)
		}
	case !s.recordsAllowed(req.Action, req.Resource.Kind):
	case code != codes.OK && (!stream || code != codes.Canceled):
		if reason == "" && st.insertFailed {
			reason = "audit write failed"
		}
		s.recordDetached(ctx, grpcEvent(ctx, method, req, OutcomeFailure, reason, code))
	case stream:
		// A stream that ended normally (or was closed by the client).
		s.recordDetached(ctx, grpcEvent(ctx, method, req, OutcomeSuccess, "", codes.OK))
	}
}

func grpcEvent(ctx context.Context, method string, req *authz.Request, outcome, reason string, code codes.Code) Event {
	e := Event{
		Outcome:   outcome,
		Reason:    reason,
		Transport: TransportGRPC,
		GRPC:      GRPCInfo{Method: method},
		Actor:     Actor{Type: ActorAnonymous},
	}
	if outcome != OutcomeSuccess {
		e.GRPC.Code = code.String()
	}
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		e.Source.IP = remoteIP(p.Addr.String())
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if ua := md.Get("user-agent"); len(ua) > 0 {
			e.Source.UserAgent = ua[0]
		}
		if tp := md.Get("traceparent"); len(tp) > 0 {
			e.TraceID = traceIDFromHeader(tp[0])
		}
	}
	if req != nil {
		e.Action = req.Action
		e.Resource = Resource{Surface: req.Resource.Surface, Kind: req.Resource.Kind, ID: req.Resource.ID, Parent: req.Resource.Parent}
		e.Actor = actorFromSubject(req.Subject)
	}
	return e
}

// grpcCode maps an error to its status code, treating raw context errors
// (a handler returning ctx.Err()) as Canceled or DeadlineExceeded.
func grpcCode(err error) codes.Code {
	if st, ok := status.FromError(err); ok {
		return st.Code()
	}
	return status.FromContextError(err).Code()
}

func bypassed(method string, bypass []string) bool {
	for _, prefix := range bypass {
		if prefix != "" && strings.HasPrefix(method, prefix) {
			return true
		}
	}
	return false
}
