package auth

import (
	"fmt"
	"net/http"

	"github.com/openshift-online/rh-trex-ai/pkg/config"
	"google.golang.org/grpc"
)

// AuthenticationStrategy defines the strategy for authentication
type AuthenticationStrategy int

const (
	AuthStrategyNone AuthenticationStrategy = iota
	AuthStrategyJWT
	AuthStrategyBearer
	AuthStrategyBoth // Both JWT and Bearer are accepted
)

// AuthMiddlewareBuilder creates authentication middleware based on configuration
type AuthMiddlewareBuilder struct {
	authConfig *config.AuthConfig
	strategy   AuthenticationStrategy
}

// NewAuthMiddlewareBuilder creates a new authentication middleware builder
func NewAuthMiddlewareBuilder(authConfig *config.AuthConfig) *AuthMiddlewareBuilder {
	var strategy AuthenticationStrategy
	if authConfig.EnableJWT && authConfig.EnableBearer {
		strategy = AuthStrategyBoth
	} else if authConfig.EnableJWT {
		strategy = AuthStrategyJWT
	} else if authConfig.EnableBearer {
		strategy = AuthStrategyBearer
	} else {
		strategy = AuthStrategyNone
	}
	
	return &AuthMiddlewareBuilder{
		authConfig: authConfig,
		strategy:   strategy,
	}
}

// BuildHTTPMiddleware creates HTTP authentication middleware based on the configured strategy
func (b *AuthMiddlewareBuilder) BuildHTTPMiddleware() (func(http.Handler) http.Handler, error) {
	switch b.strategy {
	case AuthStrategyNone:
		return func(next http.Handler) http.Handler { return next }, nil
		
	case AuthStrategyJWT:
		jwtMiddleware, err := NewAuthMiddleware()
		if err != nil {
			return nil, fmt.Errorf("failed to create JWT middleware: %w", err)
		}
		return jwtMiddleware.AuthenticateAccountJWT, nil
		
	case AuthStrategyBearer:
		if err := b.authConfig.Validate(); err != nil {
			return nil, fmt.Errorf("bearer token configuration invalid: %w", err)
		}
		return BearerTokenMiddleware(b.authConfig.BearerToken, b.authConfig.BypassPaths), nil
		
	case AuthStrategyBoth:
		// Create a middleware that accepts both JWT and Bearer tokens
		if err := b.authConfig.Validate(); err != nil {
			return nil, fmt.Errorf("bearer token configuration invalid: %w", err)
		}
		
		jwtMiddleware, err := NewAuthMiddleware()
		if err != nil {
			return nil, fmt.Errorf("failed to create JWT middleware: %w", err)
		}
		
		return b.buildDualAuthMiddleware(jwtMiddleware, b.authConfig.BearerToken, b.authConfig.BypassPaths), nil
		
	default:
		return nil, fmt.Errorf("unknown authentication strategy: %v", b.strategy)
	}
}

// BuildGRPCInterceptors creates gRPC authentication interceptors based on the configured strategy
func (b *AuthMiddlewareBuilder) BuildGRPCInterceptors() (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor, error) {
	switch b.strategy {
	case AuthStrategyNone:
		return nil, nil, nil
		
	case AuthStrategyJWT:
		// JWT for gRPC is handled by the existing AuthUnaryInterceptor in grpc_server.go
		return nil, nil, nil
		
	case AuthStrategyBearer:
		if err := b.authConfig.Validate(); err != nil {
			return nil, nil, fmt.Errorf("bearer token configuration invalid: %w", err)
		}
		return BearerTokenUnaryInterceptor(b.authConfig.BearerToken, b.authConfig.BypassMethods),
			   BearerTokenStreamInterceptor(b.authConfig.BearerToken, b.authConfig.BypassMethods),
			   nil
			   
	case AuthStrategyBoth:
		// For gRPC, we currently support either JWT or Bearer, not both simultaneously
		// This could be enhanced in the future to support dual auth
		return nil, nil, fmt.Errorf("dual authentication (JWT + Bearer) not yet supported for gRPC")
		
	default:
		return nil, nil, fmt.Errorf("unknown authentication strategy: %v", b.strategy)
	}
}

// buildDualAuthMiddleware creates middleware that accepts both JWT and Bearer tokens
func (b *AuthMiddlewareBuilder) buildDualAuthMiddleware(jwtMiddleware JWTMiddleware, bearerToken string, bypassPaths []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if this path should bypass authentication
			for _, path := range bypassPaths {
				if r.URL.Path == path || (len(r.URL.Path) > len(path) && r.URL.Path[:len(path)] == path) {
					next.ServeHTTP(w, r)
					return
				}
			}
			
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}
			
			// Try Bearer token first (simpler validation)
			if bearerToken != "" && (len(authHeader) > 7 && (authHeader[:7] == "Bearer " || authHeader[:7] == "bearer ")) {
				// Use bearer token middleware
				bearerMiddleware := BearerTokenMiddleware(bearerToken, bypassPaths)
				bearerMiddleware(next).ServeHTTP(w, r)
				return
			}
			
			// Fall back to JWT authentication
			jwtMiddleware.AuthenticateAccountJWT(next).ServeHTTP(w, r)
		})
	}
}

// GetStrategy returns the current authentication strategy
func (b *AuthMiddlewareBuilder) GetStrategy() AuthenticationStrategy {
	return b.strategy
}