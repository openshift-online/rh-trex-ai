package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/openshift-online/rh-trex-ai/pkg/logger"
)

// BearerTokenMiddleware creates HTTP middleware for bearer token authentication
// This provides a simple alternative to JWT/OIDC for internal services
func BearerTokenMiddleware(expectedToken string, bypassPaths []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication for bypass paths
			for _, path := range bypassPaths {
				if strings.HasPrefix(r.URL.Path, path) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Extract Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			// Extract bearer token
			token := strings.TrimPrefix(authHeader, "Bearer ")
			token = strings.TrimPrefix(token, "bearer ")
			
			if token == authHeader { // No "Bearer " prefix found
				http.Error(w, "Bearer token required", http.StatusUnauthorized)
				return
			}

			// Constant-time comparison to prevent timing attacks
			if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
				log := logger.NewLogger(r.Context())
				log.Infof("Invalid bearer token provided, length: %d", len(token))
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			// Token is valid, continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// DefaultBypassPaths returns the standard list of paths that should bypass authentication
func DefaultBypassPaths() []string {
	return []string{
		"/healthcheck",
		"/metrics",
		"/api/rh-trex/v1/openapi",
		"/openapi",
	}
}

// ExtendBypassPaths adds additional paths to the default bypass list
func ExtendBypassPaths(additionalPaths ...string) []string {
	paths := DefaultBypassPaths()
	return append(paths, additionalPaths...)
}