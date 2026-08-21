package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/openshift-online/rh-trex-ai/pkg/server/grpcutil"
)

func TestAuthenticateGRPCRequestValidatesExpectedClaims(t *testing.T) {
	const (
		kid      = "grpc-auth-kid"
		issuer   = "https://issuer.example.com/realms/example"
		audience = "example-api"
	)

	privateKey, provider := testGRPCKeyProvider(t, kid)

	tests := []struct {
		name        string
		claims      jwt.MapClaims
		expectError bool
	}{
		{
			name: "valid claims",
			claims: jwt.MapClaims{
				"preferred_username": "test-user",
				"iss":                issuer,
				"aud":                []interface{}{"account", audience},
				"exp":                time.Now().Add(time.Hour).Unix(),
			},
		},
		{
			name: "wrong issuer",
			claims: jwt.MapClaims{
				"preferred_username": "test-user",
				"iss":                "https://attacker.example.com/realms/example",
				"aud":                audience,
				"exp":                time.Now().Add(time.Hour).Unix(),
			},
			expectError: true,
		},
		{
			name: "wrong audience",
			claims: jwt.MapClaims{
				"preferred_username": "test-user",
				"iss":                issuer,
				"aud":                "another-api",
				"exp":                time.Now().Add(time.Hour).Unix(),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, tt.claims)
			token.Header["kid"] = kid
			tokenString, err := token.SignedString(privateKey)
			if err != nil {
				t.Fatalf("SignedString() unexpected error: %v", err)
			}

			ctx := metadata.NewIncomingContext(
				context.Background(),
				metadata.Pairs("authorization", "Bearer "+tokenString),
			)
			username, err := authenticateGRPCRequest(ctx, provider, issuer, audience)
			if tt.expectError {
				if status.Code(err) != codes.Unauthenticated {
					t.Fatalf("authenticateGRPCRequest() code = %v, want %v", status.Code(err), codes.Unauthenticated)
				}
				return
			}
			if err != nil {
				t.Fatalf("authenticateGRPCRequest() unexpected error: %v", err)
			}
			if username != "test-user" {
				t.Errorf("username = %q, want %q", username, "test-user")
			}
		})
	}
}

func testGRPCKeyProvider(t *testing.T, kid string) (*rsa.PrivateKey, *grpcutil.JWKKeyProvider) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() unexpected error: %v", err)
	}

	n := base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes())
	jwks := fmt.Sprintf(`{"keys":[{"kid":%q,"kty":"RSA","alg":"RS256","use":"sig","n":%q,"e":%q}]}`, kid, n, e)
	path := filepath.Join(t.TempDir(), "jwks.json")
	if err := os.WriteFile(path, []byte(jwks), 0600); err != nil {
		t.Fatalf("WriteFile() unexpected error: %v", err)
	}

	return privateKey, grpcutil.NewJWKKeyProvider(nil, path)
}
