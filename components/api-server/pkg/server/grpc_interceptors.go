package server

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/auth"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/logger"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/server/grpcutil"
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
				slog.ErrorContext(ctx, "gRPC panic", "method", info.FullMethod, "panic", r, "stack", string(debug.Stack()))
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

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		code := status.Code(err)
		slog.InfoContext(ctx, "gRPC call", "method", info.FullMethod, "code", code.String(), "duration", duration)

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

// TransactionUnaryInterceptor runs each unary call in a database transaction.
// The transaction is rolled back when the handler returns an error or panics.
// Otherwise it is committed, unless something called db.MarkForRollback.
func TransactionUnaryInterceptor(sessionFactory db.SessionFactory) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		ctx, txErr := db.NewContext(ctx, sessionFactory)
		if txErr != nil {
			slog.ErrorContext(ctx, "Failed to create DB transaction for gRPC call", "method", info.FullMethod, "error", txErr)
			return nil, status.Error(codes.Internal, "internal database error")
		}
		defer func() {
			if p := recover(); p != nil {
				db.MarkForRollback(ctx, fmt.Errorf("panic: %v", p))
				db.Resolve(ctx)
				panic(p)
			}
			if err != nil {
				db.MarkForRollback(ctx, err)
			}
			db.Resolve(ctx)
		}()

		return handler(ctx, req)
	}
}

func AuthUnaryInterceptor(env *environments.Env, keyProvider *grpcutil.JWKKeyProvider) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		authConfig := env.Config.GetAuthConfig()
		if !authConfig.EnableJWT {
			// No verified identity: use the local development identity, as REST does.
			return handler(auth.SetUsernameContext(ctx, devUsername), req)
		}

		if info.FullMethod == "/grpc.health.v1.Health/Check" ||
			info.FullMethod == "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo" {
			return handler(ctx, req)
		}

		username, token, err := authenticateGRPCToken(ctx, keyProvider, authConfig.JWTIssuer, authConfig.JWTAudience)
		if err != nil {
			return nil, err
		}

		ctx = withVerifiedToken(auth.SetUsernameContext(ctx, username), token)

		return handler(ctx, req)
	}
}

func RecoveryStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(ss.Context(), "gRPC stream panic", "method", info.FullMethod, "panic", r, "stack", string(debug.Stack()))
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
		slog.InfoContext(ctx, "gRPC stream started", "method", info.FullMethod)

		wrapped := &wrappedServerStream{ServerStream: ss, ctx: ctx}
		err := handler(srv, wrapped)

		duration := time.Since(start)
		code := status.Code(err)
		slog.InfoContext(ctx, "gRPC stream ended", "method", info.FullMethod, "code", code.String(), "duration", duration)

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
		authConfig := env.Config.GetAuthConfig()
		if !authConfig.EnableJWT {
			ctx := auth.SetUsernameContext(ss.Context(), devUsername)
			return handler(srv, &wrappedServerStream{ServerStream: ss, ctx: ctx})
		}

		if info.FullMethod == "/grpc.health.v1.Health/Check" ||
			info.FullMethod == "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo" {
			return handler(srv, ss)
		}

		username, token, err := authenticateGRPCToken(ss.Context(), keyProvider, authConfig.JWTIssuer, authConfig.JWTAudience)
		if err != nil {
			return err
		}

		ctx := withVerifiedToken(auth.SetUsernameContext(ss.Context(), username), token)
		wrapped := &wrappedServerStream{ServerStream: ss, ctx: ctx}

		return handler(srv, wrapped)
	}
}

// devUsername is the identity used when JWT authentication is disabled. It
// matches auth.MiddlewareMock on REST.
const devUsername = "dev-user"

// withVerifiedToken stores the verified token so authorization can read its
// claims, as the REST JWT handler does.
func withVerifiedToken(ctx context.Context, token *jwt.Token) context.Context {
	return context.WithValue(ctx, auth.ContextAuthKey, token)
}

func authenticateGRPCRequest(ctx context.Context, keyProvider *grpcutil.JWKKeyProvider, expectedIssuer, expectedAudience string) (string, error) {
	username, _, err := authenticateGRPCToken(ctx, keyProvider, expectedIssuer, expectedAudience)
	return username, err
}

func authenticateGRPCToken(ctx context.Context, keyProvider *grpcutil.JWKKeyProvider, expectedIssuer, expectedAudience string) (string, *jwt.Token, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		return "", nil, status.Error(codes.Unauthenticated, "missing authorization token")
	}

	tokenStr := strings.TrimPrefix(authHeader[0], "Bearer ")
	tokenStr = strings.TrimPrefix(tokenStr, "bearer ")

	if keyProvider == nil {
		return "", nil, status.Error(codes.Unauthenticated, "jwt key provider is not configured")
	}

	jwtToken, err := jwt.ParseWithClaims(tokenStr, jwt.MapClaims{}, keyProvider.KeyFunc)
	if err != nil {
		return "", nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}
	if err := auth.ValidateJWTClaims(jwtToken, expectedIssuer, expectedAudience); err != nil {
		return "", nil, status.Errorf(codes.Unauthenticated, "invalid token claims: %v", err)
	}

	claims, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok {
		return "", nil, status.Error(codes.Unauthenticated, "invalid token claims")
	}

	username, _ := claims["username"].(string)
	if username == "" {
		username, _ = claims["preferred_username"].(string)
	}
	if username == "" {
		username, _ = claims["sub"].(string)
	}
	if username == "" {
		return "", nil, status.Error(codes.Unauthenticated, "token missing username claim")
	}

	return username, jwtToken, nil
}
