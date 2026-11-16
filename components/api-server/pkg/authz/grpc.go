package authz

import (
	"context"
	"log/slog"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCResource is the kind and action of one gRPC method.
type GRPCResource struct {
	Kind   string
	Action string
}

// GRPCResources maps a full method name ("/pkg.Service/Method") to its resource.
// Each gRPC service declares one when it registers.
type GRPCResources map[string]GRPCResource

// GRPCMethodLookup finds the declared resource of a full method name.
type GRPCMethodLookup func(fullMethod string) (GRPCResource, bool)

// UnaryInterceptor authorizes unary calls. Methods matching a bypass prefix
// skip authorization. Methods with no declared resource are denied.
func UnaryInterceptor(a Authorizer, lookup GRPCMethodLookup, bypass []string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if bypassed(info.FullMethod, bypass) {
			return handler(ctx, req)
		}
		res, ok := lookup(info.FullMethod)
		if !ok {
			return nil, undeclared(ctx, info.FullMethod)
		}
		ctx, subject, err := prepare(ctx, a)
		observeRequest(ctx, grpcRequest(subject, res, info.FullMethod), false)
		if err != nil {
			return nil, unavailable(ctx, err)
		}
		if err := authorizeGRPC(ctx, a, subject, res, info.FullMethod, req); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamInterceptor authorizes a stream when it opens (GRPCMsg nil) and again
// for every message the server receives.
func StreamInterceptor(a Authorizer, lookup GRPCMethodLookup, bypass []string) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if bypassed(info.FullMethod, bypass) {
			return handler(srv, ss)
		}
		res, ok := lookup(info.FullMethod)
		if !ok {
			return undeclared(ss.Context(), info.FullMethod)
		}
		ctx, subject, err := prepare(ss.Context(), a)
		observeRequest(ctx, grpcRequest(subject, res, info.FullMethod), false)
		if err != nil {
			return unavailable(ctx, err)
		}
		if err := authorizeGRPC(ctx, a, subject, res, info.FullMethod, nil); err != nil {
			return err
		}
		return handler(srv, &authorizedStream{
			ServerStream: ss, ctx: ctx, a: a, subject: subject, res: res, method: info.FullMethod,
		})
	}
}

type authorizedStream struct {
	grpc.ServerStream
	ctx     context.Context
	a       Authorizer
	subject Subject
	res     GRPCResource
	method  string
}

func (s *authorizedStream) Context() context.Context { return s.ctx }

func (s *authorizedStream) RecvMsg(m interface{}) error {
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return err
	}
	return authorizeGRPC(s.ctx, s.a, s.subject, s.res, s.method, m)
}

func grpcRequest(subject Subject, res GRPCResource, method string) Request {
	return Request{
		Subject:    subject,
		Action:     res.Action,
		Resource:   Resource{Kind: res.Kind},
		GRPCMethod: method,
	}
}

func authorizeGRPC(ctx context.Context, a Authorizer, subject Subject, res GRPCResource, method string, msg any) error {
	req := grpcRequest(subject, res, method)
	req.GRPCMsg = msg
	d, err := a.Authorize(ctx, req)
	if err != nil {
		return unavailable(ctx, err)
	}
	observeDecision(ctx, d)
	if d.Allowed {
		return nil
	}
	if d.HideExistence {
		return status.Error(codes.NotFound, "not found")
	}
	reason := d.Reason
	if reason == "" {
		reason = "forbidden"
	}
	return status.Error(codes.PermissionDenied, reason)
}

func bypassed(method string, bypass []string) bool {
	for _, prefix := range bypass {
		if prefix != "" && strings.HasPrefix(method, prefix) {
			return true
		}
	}
	return false
}

func undeclared(ctx context.Context, method string) error {
	slog.ErrorContext(ctx, "gRPC method has no declared resource; denying", "method", method)
	observeUndeclared(ctx, Request{Subject: subjectFromAuth(ctx), Action: "unknown", GRPCMethod: method})
	return status.Error(codes.PermissionDenied, "method has no declared resource")
}

func unavailable(ctx context.Context, err error) error {
	slog.ErrorContext(ctx, "Authorization could not decide", "error", err)
	observeError(ctx, err)
	return status.Error(codes.Unavailable, "authorization unavailable")
}

// CRUDResources is the method table for a service that follows the TRex gRPC
// conventions: Get, Create, Update and Delete<Singular>, and List and
// Watch<Plural>. service is the full service name, e.g. "rh_trex.v1.DinosaurService".
func CRUDResources(service, kind, singular, plural string) GRPCResources {
	m := func(name string) string { return "/" + service + "/" + name }
	return GRPCResources{
		m("Get" + singular):    {Kind: kind, Action: ActionGet},
		m("Create" + singular): {Kind: kind, Action: ActionCreate},
		m("Update" + singular): {Kind: kind, Action: ActionUpdate},
		m("Delete" + singular): {Kind: kind, Action: ActionDelete},
		m("List" + plural):     {Kind: kind, Action: ActionList},
		m("Watch" + plural):    {Kind: kind, Action: ActionWatch},
	}
}
