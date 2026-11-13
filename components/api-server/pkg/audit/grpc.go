package audit

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/authz"
)

// UnaryBegin is the gRPC unary hook. TRex places it before authentication
// and outside the transaction interceptor, so a recorder that writes to the
// database runs after the call has released its connection. Methods matching
// a bypass prefix are not audited.
func (s *Service) UnaryBegin(bypass []string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		if !s.Enabled() || bypassed(info.FullMethod, bypass) {
			return handler(ctx, req)
		}
		ctx, obs := authz.WithObservation(ctx)
		panicked := true
		defer func() {
			if panicked {
				s.finishGRPC(ctx, info.FullMethod, obs, codes.Internal, "panic", false)
			}
		}()
		resp, err = handler(ctx, req)
		panicked = false
		s.finishGRPC(ctx, info.FullMethod, obs, grpcCode(err), "", false)
		return resp, err
	}
}

// StreamBegin is the gRPC stream hook. A stream is recorded when it ends.
func (s *Service) StreamBegin(bypass []string) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !s.Enabled() || bypassed(info.FullMethod, bypass) {
			return handler(srv, ss)
		}
		ctx, obs := authz.WithObservation(ss.Context())
		panicked := true
		defer func() {
			if panicked {
				s.finishGRPC(ctx, info.FullMethod, obs, codes.Internal, "panic", true)
			}
		}()
		err := handler(srv, &contextStream{ServerStream: ss, ctx: ctx})
		panicked = false
		s.finishGRPC(ctx, info.FullMethod, obs, grpcCode(err), "", true)
		return err
	}
}

type contextStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (c *contextStream) Context() context.Context { return c.ctx }

func (s *Service) finishGRPC(ctx context.Context, method string, obs *authz.Observation, code codes.Code, reason string, stream bool) {
	req := obs.Request
	switch {
	case obs.Undeclared:
		s.recordFromHook(ctx, grpcEvent(ctx, method, req, OutcomeDenied, "method has no declared resource", code))
	case obs.Err != nil:
		s.recordFromHook(ctx, grpcEvent(ctx, method, req, OutcomeError, "authorization unavailable", code))
	case obs.Decided && !obs.Decision.Allowed:
		s.recordFromHook(ctx, grpcEvent(ctx, method, req, OutcomeDenied, denyReason(obs.Decision), code))
	case req == nil:
		if code == codes.Unauthenticated {
			e := grpcEvent(ctx, method, nil, OutcomeUnauthenticated, "authentication failed", code)
			e.Action = "unknown"
			s.recordFromHook(ctx, e)
		}
	case !s.recordsAllowed(req.Action, req.Resource.Kind):
	case code != codes.OK && (!stream || code != codes.Canceled):
		s.recordFromHook(ctx, grpcEvent(ctx, method, req, OutcomeFailure, reason, code))
	default:
		// Success, or a stream the client closed.
		s.recordFromHook(ctx, grpcEvent(ctx, method, req, OutcomeSuccess, "", code))
	}
}

func grpcEvent(ctx context.Context, method string, req *authz.Request, outcome, reason string, code codes.Code) Event {
	e := Event{
		Outcome:   outcome,
		Reason:    reason,
		Transport: TransportGRPC,
		GRPC:      GRPCInfo{Method: method, Code: code.String()},
		Actor:     Actor{Type: ActorAnonymous},
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
