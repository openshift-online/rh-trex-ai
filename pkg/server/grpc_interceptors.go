package server

import (
	"context"
	"runtime/debug"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/golang-jwt/jwt/v4"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/openshift-online/rh-trex-ai/pkg/auth"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
	"github.com/openshift-online/rh-trex-ai/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/pkg/logger"
)

var (
	grpcRequestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Total number of gRPC requests",
		},
		[]string{"method", "code"},
	)
	grpcRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "gRPC request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)
)

func init() {
	prometheus.MustRegister(grpcRequestCount)
	prometheus.MustRegister(grpcRequestDuration)
}

func RecoveryUnaryInterceptor(sentryTimeout time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				glog.Errorf("gRPC panic in %s: %v\n%s", info.FullMethod, r, debug.Stack())
				sentry.CurrentHub().Recover(r)
				sentry.Flush(sentryTimeout)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

func LoggingUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		ctx = logger.WithOpID(ctx)
		log := logger.NewOCMLogger(ctx)

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		code := status.Code(err)
		log.Infof("gRPC %s %s %s", info.FullMethod, code, duration)

		return resp, err
	}
}

func MetricsUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)
		code := status.Code(err)

		grpcRequestCount.WithLabelValues(info.FullMethod, code.String()).Inc()
		grpcRequestDuration.WithLabelValues(info.FullMethod).Observe(duration.Seconds())

		return resp, err
	}
}

func TransactionUnaryInterceptor(sessionFactory db.SessionFactory) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx, err := db.NewContext(ctx, sessionFactory)
		if err != nil {
			glog.Errorf("Failed to create DB transaction for gRPC call %s: %v", info.FullMethod, err)
			return nil, status.Error(codes.Internal, "internal database error")
		}
		defer func() { db.Resolve(ctx) }()

		return handler(ctx, req)
	}
}

func AuthUnaryInterceptor(env *environments.Env) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !env.Config.Server.EnableJWT {
			return handler(ctx, req)
		}

		if info.FullMethod == "/grpc.health.v1.Health/Check" ||
			info.FullMethod == "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo" {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization token")
		}

		token := strings.TrimPrefix(authHeader[0], "Bearer ")
		token = strings.TrimPrefix(token, "bearer ")

		parser := jwt.NewParser()
		jwtToken, _, err := parser.ParseUnverified(token, jwt.MapClaims{})
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid token format")
		}

		claims, ok := jwtToken.Claims.(jwt.MapClaims)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "invalid token claims")
		}

		username, _ := claims["username"].(string)
		if username == "" {
			username, _ = claims["preferred_username"].(string)
		}
		if username == "" {
			return nil, status.Error(codes.Unauthenticated, "token missing username claim")
		}

		ctx = auth.SetUsernameContext(ctx, username)

		return handler(ctx, req)
	}
}

func RecoveryStreamInterceptor(sentryTimeout time.Duration) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				glog.Errorf("gRPC stream panic in %s: %v\n%s", info.FullMethod, r, debug.Stack())
				sentry.CurrentHub().Recover(r)
				sentry.Flush(sentryTimeout)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(srv, ss)
	}
}

func LoggingStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		log := logger.NewOCMLogger(ss.Context())
		log.Infof("gRPC stream started %s", info.FullMethod)

		err := handler(srv, ss)

		duration := time.Since(start)
		code := status.Code(err)
		log.Infof("gRPC stream ended %s %s %s", info.FullMethod, code, duration)

		return err
	}
}

func MetricsStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)
		duration := time.Since(start)
		code := status.Code(err)

		grpcRequestCount.WithLabelValues(info.FullMethod, code.String()).Inc()
		grpcRequestDuration.WithLabelValues(info.FullMethod).Observe(duration.Seconds())

		return err
	}
}

func AuthStreamInterceptor(env *environments.Env) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !env.Config.Server.EnableJWT {
			return handler(srv, ss)
		}

		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			return status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return status.Error(codes.Unauthenticated, "missing authorization token")
		}

		token := strings.TrimPrefix(authHeader[0], "Bearer ")
		token = strings.TrimPrefix(token, "bearer ")

		parser := jwt.NewParser()
		jwtToken, _, err := parser.ParseUnverified(token, jwt.MapClaims{})
		if err != nil {
			return status.Error(codes.Unauthenticated, "invalid token format")
		}

		claims, ok := jwtToken.Claims.(jwt.MapClaims)
		if !ok {
			return status.Error(codes.Unauthenticated, "invalid token claims")
		}

		username, _ := claims["username"].(string)
		if username == "" {
			username, _ = claims["preferred_username"].(string)
		}
		if username == "" {
			return status.Error(codes.Unauthenticated, "token missing username claim")
		}

		return handler(srv, ss)
	}
}
