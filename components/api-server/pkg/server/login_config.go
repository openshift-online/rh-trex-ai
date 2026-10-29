package server

import (
	"encoding/json"
	"net/http"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
)

// loginConfigHandler exposes only the configured issuer, never the JWK URL or
// client credentials. Signing keys may be fetched through an internal service.
func loginConfigHandler(authConfig *config.AuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authConfig.EnableJWT || authConfig.JWTIssuer == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			IssuerURL string `json:"issuer_url"`
		}{IssuerURL: authConfig.JWTIssuer})
	}
}
