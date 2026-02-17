package server

import (
	"net"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/openshift-online/rh-trex-ai/pkg/environments"
)

type grpcAPIServer struct {
	grpcServer *grpc.Server
	env        *environments.Env
}

var _ Server = &grpcAPIServer{}

func NewDefaultGRPCServer(env *environments.Env) Server {
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			RecoveryUnaryInterceptor(env.Config.Sentry.Timeout),
			LoggingUnaryInterceptor(),
			MetricsUnaryInterceptor(),
			TransactionUnaryInterceptor(env.Database.SessionFactory),
			AuthUnaryInterceptor(env),
		),
		grpc.ChainStreamInterceptor(
			RecoveryStreamInterceptor(env.Config.Sentry.Timeout),
			LoggingStreamInterceptor(),
			MetricsStreamInterceptor(),
			AuthStreamInterceptor(env),
		),
	}

	if env.Config.GRPC.EnableTLS {
		creds, err := credentials.NewServerTLSFromFile(
			env.Config.GRPC.TLSCertFile,
			env.Config.GRPC.TLSKeyFile,
		)
		if err != nil {
			glog.Fatalf("Failed to load gRPC TLS credentials: %v", err)
		}
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
		Check(err, "gRPC server terminated with errors", s.env.Config.Sentry.Timeout)
	}
	glog.Info("gRPC server terminated")
}

func (s *grpcAPIServer) Stop() error {
	glog.Info("gRPC server shutting down gracefully")
	s.grpcServer.GracefulStop()
	return nil
}
