package auth

import (
	"testing"

	"github.com/golang-jwt/jwt/v4"
)

func TestValidateJWTClaims(t *testing.T) {
	const (
		expectedIssuer   = "https://issuer.example.com/realms/example"
		expectedAudience = "example-api"
	)

	tests := []struct {
		name             string
		claims           jwt.MapClaims
		issuer           string
		audience         string
		expectValidation bool
	}{
		{
			name:             "validation disabled",
			claims:           jwt.MapClaims{},
			expectValidation: true,
		},
		{
			name: "matching issuer and string audience",
			claims: jwt.MapClaims{
				"iss": expectedIssuer,
				"aud": expectedAudience,
			},
			issuer:           expectedIssuer,
			audience:         expectedAudience,
			expectValidation: true,
		},
		{
			name: "matching audience in array",
			claims: jwt.MapClaims{
				"iss": expectedIssuer,
				"aud": []interface{}{"account", expectedAudience},
			},
			issuer:           expectedIssuer,
			audience:         expectedAudience,
			expectValidation: true,
		},
		{
			name: "missing issuer",
			claims: jwt.MapClaims{
				"aud": expectedAudience,
			},
			issuer:   expectedIssuer,
			audience: expectedAudience,
		},
		{
			name: "wrong issuer",
			claims: jwt.MapClaims{
				"iss": "https://attacker.example.com/realms/example",
				"aud": expectedAudience,
			},
			issuer:   expectedIssuer,
			audience: expectedAudience,
		},
		{
			name: "missing audience",
			claims: jwt.MapClaims{
				"iss": expectedIssuer,
			},
			issuer:   expectedIssuer,
			audience: expectedAudience,
		},
		{
			name: "wrong audience",
			claims: jwt.MapClaims{
				"iss": expectedIssuer,
				"aud": "another-api",
			},
			issuer:   expectedIssuer,
			audience: expectedAudience,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &jwt.Token{Claims: tt.claims, Valid: true}
			err := ValidateJWTClaims(token, tt.issuer, tt.audience)
			if tt.expectValidation && err != nil {
				t.Fatalf("ValidateJWTClaims() unexpected error: %v", err)
			}
			if !tt.expectValidation && err == nil {
				t.Fatal("ValidateJWTClaims() expected an error")
			}
		})
	}
}

func TestValidateJWTClaimsRejectsNilToken(t *testing.T) {
	if err := ValidateJWTClaims(nil, "", ""); err == nil {
		t.Fatal("ValidateJWTClaims() expected an error for a nil token")
	}
}

func TestValidateJWTClaimsRejectsInvalidToken(t *testing.T) {
	token := &jwt.Token{Claims: jwt.MapClaims{}, Valid: false}
	if err := ValidateJWTClaims(token, "", ""); err == nil {
		t.Fatal("ValidateJWTClaims() expected an error for an invalid token")
	}
}
