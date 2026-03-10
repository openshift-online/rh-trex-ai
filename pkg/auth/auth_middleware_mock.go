package auth

import (
	"net/http"
)

type MiddlewareMock struct{}

var _ JWTMiddleware = &MiddlewareMock{}

func (a *MiddlewareMock) AuthenticateAccountJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := SetUsernameContext(r.Context(), "dev-user")
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
