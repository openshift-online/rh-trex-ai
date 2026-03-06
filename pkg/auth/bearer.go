package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/openshift-online/rh-trex-ai/pkg/logger"
)

func BearerTokenMiddleware(expectedToken string, bypassPaths []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, path := range bypassPaths {
				if strings.HasPrefix(r.URL.Path, path) {
					next.ServeHTTP(w, r)
					return
				}
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			token = strings.TrimPrefix(token, "bearer ")

			if token == authHeader {
				http.Error(w, "Bearer token required", http.StatusUnauthorized)
				return
			}

			if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
				log := logger.NewLogger(r.Context())
				log.Infof("Invalid bearer token provided, length: %d", len(token))
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func DefaultBypassPaths() []string {
	return []string{
		"/healthcheck",
		"/metrics",
		"/api/rh-trex/v1/openapi",
		"/openapi",
	}
}

func ExtendBypassPaths(additionalPaths ...string) []string {
	paths := DefaultBypassPaths()
	return append(paths, additionalPaths...)
}
