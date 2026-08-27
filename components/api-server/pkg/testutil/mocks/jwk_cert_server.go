package mocks

import (
	"crypto"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/mendsley/gojwk"
)

const (
	certEndpoint  = "/auth/realms/rhd/protocol/openid-connect/certs"
	tokenEndpoint = "/auth/realms/rhd/protocol/openid-connect/token"
)

func NewJWKCertServerMock(t *testing.T, pubKey crypto.PublicKey, jwkKID string, jwkAlg string) (url string, teardown func() error) {
	certHandler := http.NewServeMux()
	certHandler.HandleFunc(certEndpoint,
		func(w http.ResponseWriter, r *http.Request) {
			pubjwk, err := gojwk.PublicKey(pubKey)
			if err != nil {
				t.Errorf("Unable to generate public jwk: %s", err)
				return
			}
			pubjwk.Kid = jwkKID
			pubjwk.Alg = jwkAlg
			jwkBytes, err := gojwk.Marshal(pubjwk)
			if err != nil {
				t.Errorf("Unable to marshal public jwk: %s", err)
				return
			}
			_, _ = fmt.Fprintf(w, `{"keys":[%s]}`, string(jwkBytes))
		},
	)

	server := httptest.NewServer(certHandler)
	return fmt.Sprintf("%s%s", server.URL, certEndpoint), serverClose(server)
}

func NewJWKServerMock(t *testing.T, privKey *rsa.PrivateKey, pubKey crypto.PublicKey, jwkKID string, jwkAlg string, issuer string) (certURL string, tokenURL string, teardown func() error) {
	mux := http.NewServeMux()

	mux.HandleFunc(certEndpoint, func(w http.ResponseWriter, r *http.Request) {
		pubjwk, err := gojwk.PublicKey(pubKey)
		if err != nil {
			t.Errorf("Unable to generate public jwk: %s", err)
			return
		}
		pubjwk.Kid = jwkKID
		pubjwk.Alg = jwkAlg
		jwkBytes, err := gojwk.Marshal(pubjwk)
		if err != nil {
			t.Errorf("Unable to marshal public jwk: %s", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"keys":[%s]}`, string(jwkBytes))
	})

	mux.HandleFunc(tokenEndpoint, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		username := r.FormValue("username")
		if username == "" {
			username = "testuser"
		}

		claims := jwt.MapClaims{
			"iss":      issuer,
			"username": username,
			"typ":      "Bearer",
			"iat":      time.Now().Unix(),
			"exp":      time.Now().Add(1 * time.Hour).Unix(),
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tok.Header["kid"] = jwkKID
		signed, err := tok.SignedString(privKey)
		if err != nil {
			t.Errorf("Unable to sign token: %s", err)
			http.Error(w, "signing error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": signed,
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})

	server := httptest.NewServer(mux)
	return fmt.Sprintf("%s%s", server.URL, certEndpoint),
		fmt.Sprintf("%s%s", server.URL, tokenEndpoint),
		serverClose(server)
}

func serverClose(server *httptest.Server) func() error {
	return func() error {
		server.Close()
		return nil
	}
}
