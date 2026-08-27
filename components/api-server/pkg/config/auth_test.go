package config

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestAuthConfigJWTClaimFlags(t *testing.T) {
	authConfig := NewAuthConfig()
	flags := pflag.NewFlagSet("auth", pflag.ContinueOnError)
	authConfig.AddFlags(flags)

	const (
		issuer   = "https://issuer.example.com/realms/example"
		audience = "example-api"
	)
	if err := flags.Parse([]string{
		"--jwt-issuer=" + issuer,
		"--jwt-audience=" + audience,
	}); err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}

	if authConfig.JWTIssuer != issuer {
		t.Errorf("JWTIssuer = %q, want %q", authConfig.JWTIssuer, issuer)
	}
	if authConfig.JWTAudience != audience {
		t.Errorf("JWTAudience = %q, want %q", authConfig.JWTAudience, audience)
	}
}
