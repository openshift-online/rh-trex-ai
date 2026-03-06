package auth

import (
	"context"
	"crypto/subtle"
	"strings"

	"github.com/openshift-online/rh-trex-ai/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// BearerTokenUnaryInterceptor creates a unary gRPC interceptor for bearer token authentication
func BearerTokenUnaryInterceptor(expectedToken string, bypassMethods []string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Skip authentication for bypass methods
		for _, method := range bypassMethods {
			if strings.HasPrefix(info.FullMethod, method) {
				return handler(ctx, req)
			}
		}

		if err := validateBearerToken(ctx, expectedToken); err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

// BearerTokenStreamInterceptor creates a stream gRPC interceptor for bearer token authentication
func BearerTokenStreamInterceptor(expectedToken string, bypassMethods []string) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Skip authentication for bypass methods
		for _, method := range bypassMethods {
			if strings.HasPrefix(info.FullMethod, method) {
				return handler(srv, stream)
			}
		}

		if err := validateBearerToken(stream.Context(), expectedToken); err != nil {
			return err
		}

		return handler(srv, stream)
	}
}

// validateBearerToken extracts and validates the bearer token from gRPC metadata
func validateBearerToken(ctx context.Context, expectedToken string) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		return status.Error(codes.Unauthenticated, "authorization token required")
	}

	// Extract bearer token from first authorization header
	token := strings.TrimPrefix(authHeader[0], "Bearer ")
	token = strings.TrimPrefix(token, "bearer ")
	
	if token == authHeader[0] { // No "Bearer " prefix found
		return status.Error(codes.Unauthenticated, "bearer token required")
	}

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
		log := logger.NewLogger(ctx)
		log.Infof("Invalid bearer token provided in gRPC call, length: %d", len(token))
		return status.Error(codes.Unauthenticated, "invalid token")
	}

	return nil
}

// DefaultBypassMethods returns the standard list of gRPC methods that should bypass authentication
func DefaultBypassMethods() []string {
	return []string{
		"/grpc.health.v1.Health/",
		"/grpc.reflection.v1alpha.ServerReflection/",
	}
}

// ExtendBypassMethods adds additional methods to the default bypass list
func ExtendBypassMethods(additionalMethods ...string) []string {
	methods := DefaultBypassMethods()
	return append(methods, additionalMethods...)
}