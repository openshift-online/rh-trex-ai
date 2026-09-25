package server

import (
	"net"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/auth"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/server/grpcutil"
)

// Global interceptor registries for pre-auth interceptors
// Mirrors the pattern from apiserver.go for HTTP middleware
var preAuthUnaryInterceptors []grpc.UnaryServerInterceptor
var preAuthStreamInterceptors []grpc.StreamServerInterceptor

// RegisterPreAuthGRPCUnaryInterceptor registers a unary interceptor that runs before JWT auth
// This allows downstream components (like API server) to add custom authentication
func RegisterPreAuthGRPCUnaryInterceptor(interceptor grpc.UnaryServerInterceptor) {
	preAuthUnaryInterceptors = append(preAuthUnaryInterceptors, interceptor)
}

// RegisterPreAuthGRPCStreamInterceptor registers a stream interceptor that runs before JWT auth
// This allows downstream components (like API server) to add custom authentication
func RegisterPreAuthGRPCStreamInterceptor(interceptor grpc.StreamServerInterceptor) {
	preAuthStreamInterceptors = append(preAuthStreamInterceptors, interceptor)
}

// Global interceptor registries for post-auth interceptors
// These run after JWT authentication, so the caller's identity is available in the context.
// Use case: authorization/RBAC middleware that needs the authenticated username to populate
// access scopes, mirroring what HTTP post-auth middleware does for REST endpoints.
var postAuthUnaryInterceptors []grpc.UnaryServerInterceptor
var postAuthStreamInterceptors []grpc.StreamServerInterceptor

// RegisterPostAuthGRPCUnaryInterceptor registers a unary interceptor that runs after JWT auth
func RegisterPostAuthGRPCUnaryInterceptor(interceptor grpc.UnaryServerInterceptor) {
	postAuthUnaryInterceptors = append(postAuthUnaryInterceptors, interceptor)
}

// RegisterPostAuthGRPCStreamInterceptor registers a stream interceptor that runs after JWT auth
func RegisterPostAuthGRPCStreamInterceptor(interceptor grpc.StreamServerInterceptor) {
	postAuthStreamInterceptors = append(postAuthStreamInterceptors, interceptor)
}

type grpcAPIServer struct {
	grpcServer *grpc.Server
	env        *environments.Env
}

var _ Server = &grpcAPIServer{}

func NewDefaultGRPCServer(env *environments.Env) Server {
	// Set up authentication based on configuration
	authConfig := env.Config.GetAuthConfig()
	var keyProvider *grpcutil.JWKKeyProvider

	if authConfig.EnableJWT {
		grpcJwkURLs := authConfig.GRPCJwkCertURLs
		if len(grpcJwkURLs) == 0 {
			grpcJwkURLs = authConfig.JwkCertURLs
		}
		grpcJwkFile := authConfig.GRPCJwkCertFile
		if grpcJwkFile == "" {
			grpcJwkFile = authConfig.JwkCertFile
		}
		keyProvider = grpcutil.NewJWKKeyProvider(grpcJwkURLs, grpcJwkFile)
	}

	// Auto-register bearer token interceptors if configured
	if authConfig.EnableBearer && authConfig.BearerToken != "" {
		bearerUnary := auth.BearerTokenUnaryInterceptor(authConfig.BearerToken, authConfig.BypassMethods)
		bearerStream := auth.BearerTokenStreamInterceptor(authConfig.BearerToken, authConfig.BypassMethods)

		RegisterPreAuthGRPCUnaryInterceptor(bearerUnary)
		RegisterPreAuthGRPCStreamInterceptor(bearerStream)
	}

	// Build interceptor chains with pre-auth interceptors running BEFORE JWT auth
	unaryChain := []grpc.UnaryServerInterceptor{
		RecoveryUnaryInterceptor(),
		LoggingUnaryInterceptor(),
		MetricsUnaryInterceptor(),
		TransactionUnaryInterceptor(env.Database.SessionFactory),
	}
	// Add pre-auth interceptors before JWT auth
	unaryChain = append(unaryChain, preAuthUnaryInterceptors...)
	unaryChain = append(unaryChain, AuthUnaryInterceptor(env, keyProvider))
	// Add post-auth interceptors after JWT auth (caller identity available in context)
	unaryChain = append(unaryChain, postAuthUnaryInterceptors...)

	streamChain := []grpc.StreamServerInterceptor{
		RecoveryStreamInterceptor(),
		LoggingStreamInterceptor(),
		MetricsStreamInterceptor(),
	}
	// Add pre-auth interceptors before JWT auth
	streamChain = append(streamChain, preAuthStreamInterceptors...)
	streamChain = append(streamChain, AuthStreamInterceptor(env, keyProvider))
	// Add post-auth interceptors after JWT auth (caller identity available in context)
	streamChain = append(streamChain, postAuthStreamInterceptors...)

	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unaryChain...),
		grpc.ChainStreamInterceptor(streamChain...),
	}

	// Shared --enable-tls takes precedence; otherwise --grpc-enable-tls enables
	// TLS on the gRPC listener alone (see grpc_tls.go).
	creds, err := grpcTransportCredentials(env.Config.TLS, env.Config.GRPC)
	if err != nil {
		glog.Fatalf("Unable to configure gRPC TLS: %v", err)
	}
	if creds != nil {
		opts = append(opts, grpc.Creds(creds))
	}

	s := &grpcAPIServer{
		grpcServer: grpc.NewServer(opts...),
		env:        env,
	}

	LoadDiscoveredGRPCServices(s.grpcServer, &env.Services)

	healthServer := health.NewServer()
	healthgrpc.RegisterHealthServer(s.grpcServer, healthServer)

	reflection.Register(s.grpcServer)

	return s
}

func (s *grpcAPIServer) Start() {
	listener, err := s.Listen()
	if err != nil {
		glog.Fatalf("Unable to start gRPC server: %v", err)
	}
	glog.Infof("gRPC server listening at %s", s.env.Config.GRPC.BindAddress)
	s.Serve(listener)
}

func (s *grpcAPIServer) Listen() (net.Listener, error) {
	return net.Listen("tcp", s.env.Config.GRPC.BindAddress)
}

func (s *grpcAPIServer) Serve(listener net.Listener) {
	if err := s.grpcServer.Serve(listener); err != nil {
		Check(err, "gRPC server terminated with errors")
	}
	glog.Info("gRPC server terminated")
}

func (s *grpcAPIServer) Stop() error {
	glog.Info("gRPC server shutting down gracefully")
	s.grpcServer.GracefulStop()
	return nil
}
