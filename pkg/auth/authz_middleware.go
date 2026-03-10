package auth

import (
	"net/http"

	"github.com/openshift-online/rh-trex-ai/pkg/config"
)

type AuthorizationMiddleware interface {
	AuthorizeApi(next http.Handler) http.Handler
}

type authzMiddleware struct {
	authConfig *config.AuthConfig
}

var _ AuthorizationMiddleware = &authzMiddleware{}

func NewAuthzMiddleware(authConfig *config.AuthConfig) AuthorizationMiddleware {
	return &authzMiddleware{authConfig: authConfig}
}

func (a *authzMiddleware) AuthorizeApi(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := GetUsernameFromContext(r.Context())
		if username == "" {
			http.Error(w, "Unauthorized: missing identity", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
