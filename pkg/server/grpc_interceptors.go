package server

import (
	"context"
	"runtime/debug"
	"strings"
	"time"

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
	"github.com/openshift-online/rh-trex-ai/pkg/server/grpcutil"
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

type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context { return w.ctx }

func RecoveryUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				glog.Errorf("gRPC panic in %s: %v\n%s", info.FullMethod, r, debug.Stack())
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
		log := logger.NewLogger(ctx)

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

func AuthUnaryInterceptor(env *environments.Env, keyProvider *grpcutil.JWKKeyProvider) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		authConfig := env.Config.GetEffectiveAuthConfig()
		if !authConfig.EnableJWT {
			return handler(ctx, req)
		}

		if info.FullMethod == "/grpc.health.v1.Health/Check" ||
			info.FullMethod == "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo" {
			return handler(ctx, req)
		}

		username, err := authenticateGRPCRequest(ctx, keyProvider)
		if err != nil {
			return nil, err
		}

		ctx = auth.SetUsernameContext(ctx, username)

		return handler(ctx, req)
	}
}

func RecoveryStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				glog.Errorf("gRPC stream panic in %s: %v\n%s", info.FullMethod, r, debug.Stack())
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(srv, ss)
	}
}

func LoggingStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		ctx := logger.WithOpID(ss.Context())
		log := logger.NewLogger(ctx)
		log.Infof("gRPC stream started %s", info.FullMethod)

		wrapped := &wrappedServerStream{ServerStream: ss, ctx: ctx}
		err := handler(srv, wrapped)

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

func AuthStreamInterceptor(env *environments.Env, keyProvider *grpcutil.JWKKeyProvider) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		authConfig := env.Config.GetEffectiveAuthConfig()
		if !authConfig.EnableJWT {
			return handler(srv, ss)
		}

		username, err := authenticateGRPCRequest(ss.Context(), keyProvider)
		if err != nil {
			return err
		}

		ctx := auth.SetUsernameContext(ss.Context(), username)
		wrapped := &wrappedServerStream{ServerStream: ss, ctx: ctx}

		return handler(srv, wrapped)
	}
}

func authenticateGRPCRequest(ctx context.Context, keyProvider *grpcutil.JWKKeyProvider) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization token")
	}

	tokenStr := strings.TrimPrefix(authHeader[0], "Bearer ")
	tokenStr = strings.TrimPrefix(tokenStr, "bearer ")

	var jwtToken *jwt.Token
	var err error

	if keyProvider != nil {
		jwtToken, err = jwt.ParseWithClaims(tokenStr, jwt.MapClaims{}, keyProvider.KeyFunc)
	} else {
		parser := jwt.NewParser()
		jwtToken, _, err = parser.ParseUnverified(tokenStr, jwt.MapClaims{})
	}
	if err != nil {
		return "", status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}

	claims, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "invalid token claims")
	}

	username, _ := claims["username"].(string)
	if username == "" {
		username, _ = claims["preferred_username"].(string)
	}
	if username == "" {
		return "", status.Error(codes.Unauthenticated, "token missing username claim")
	}

	return username, nil
}
