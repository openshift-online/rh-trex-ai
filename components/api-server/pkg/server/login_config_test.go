package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/trex"
)

func TestPublicLoginConfig(t *testing.T) {
	trex.Init(trex.Config{})
	// The real outer JWT middleware fetches keys from this local service.
	// Discovery must publish the configured public issuer, not this address.
	keys := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"keys":[{"kty":"RSA","kid":"test","n":"AQAB","e":"AQAB"}]}`))
	}))
	defer keys.Close()
	for _, tc := range []struct {
		name   string
		jwt    bool
		issuer string
		status int
	}{
		{"public issuer", true, "https://login.example.com/realms/example", http.StatusOK},
		{"no issuer", true, "", http.StatusNotFound},
		{"JWT disabled", false, "https://login.example.com/realms/example", http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.NewApplicationConfig()
			cfg.Auth.EnableJWT = tc.jwt
			cfg.Auth.JWTIssuer = tc.issuer
			cfg.Auth.JwkCertURLs = []string{keys.URL}
			cfg.Auth.BearerToken = "must-not-be-exposed"
			env := &environments.Env{Config: cfg}
			// No database or credentials: exercise both middleware layers.
			server := NewDefaultAPIServer(env, []byte(`{}`)).(*defaultAPIServer)
			for _, suffix := range []string{"", "/"} {
				response := httptest.NewRecorder()
				server.httpServer.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, trex.GetConfig().BasePath+"/auth/config"+suffix, nil))
				if response.Code != tc.status {
					t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
				}
				if tc.status == http.StatusOK {
					var metadata map[string]string
					if err := json.Unmarshal(response.Body.Bytes(), &metadata); err != nil {
						t.Fatal(err)
					}
					if len(metadata) != 1 || metadata["issuer_url"] != tc.issuer {
						t.Fatalf("metadata = %v", metadata)
					}
					if response.Header().Get("Content-Type") != "application/json" {
						t.Fatal("expected JSON content type")
					}
				}
			}
			if tc.jwt {
				response := httptest.NewRecorder()
				server.httpServer.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, trex.GetConfig().BasePath+"/auth/config/private", nil))
				if response.Code != http.StatusUnauthorized {
					t.Fatalf("discovery must not exempt child paths: %d", response.Code)
				}
			}
		})
	}
}
