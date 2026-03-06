package server

import (
	"crypto/tls"
	"net"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/openshift-online/rh-trex-ai/pkg/auth"
	"github.com/openshift-online/rh-trex-ai/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/pkg/server/grpcutil"
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

type grpcAPIServer struct {
	grpcServer *grpc.Server
	env        *environments.Env
}

var _ Server = &grpcAPIServer{}

func NewDefaultGRPCServer(env *environments.Env) Server {
	// Set up authentication based on configuration
	authConfig := env.Config.GetEffectiveAuthConfig()
	var keyProvider *grpcutil.JWKKeyProvider
	
	if authConfig.EnableJWT {
		keyProvider = grpcutil.NewJWKKeyProvider(authConfig.JwkCertURL, authConfig.JwkCertFile)
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

	streamChain := []grpc.StreamServerInterceptor{
		RecoveryStreamInterceptor(),
		LoggingStreamInterceptor(),
		MetricsStreamInterceptor(),
	}
	// Add pre-auth interceptors before JWT auth
	streamChain = append(streamChain, preAuthStreamInterceptors...)
	streamChain = append(streamChain, AuthStreamInterceptor(env, keyProvider))

	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unaryChain...),
		grpc.ChainStreamInterceptor(streamChain...),
	}

	// Apply TLS configuration using the new TLS framework
	if env.Config.TLS.EnableTLS || env.Config.GRPC.EnableTLS {
		// Use new TLS framework for server configuration
		tlsConfig, err := env.Config.TLS.BuildServerTLSConfig()
		if err != nil {
			// Fall back to legacy gRPC TLS configuration if new framework fails
			if env.Config.GRPC.EnableTLS {
				creds, err := credentials.NewServerTLSFromFile(
					env.Config.GRPC.TLSCertFile,
					env.Config.GRPC.TLSKeyFile,
				)
				if err != nil {
					glog.Fatalf("Failed to load gRPC TLS credentials: %v", err)
				}
				opts = append(opts, grpc.Creds(creds))
				glog.Info("Using legacy gRPC TLS configuration")
			}
		} else if tlsConfig != nil {
			creds := credentials.NewTLS(tlsConfig)
			opts = append(opts, grpc.Creds(creds))
			glog.Infof("Using enhanced TLS configuration with minimum version %s", 
				func() string {
					switch tlsConfig.MinVersion {
					case tls.VersionTLS12:
						return "TLS 1.2"
					case tls.VersionTLS13:
						return "TLS 1.3"
					default:
						return "unknown"
					}
				}())
		}
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
