package auth

import (
	"fmt"

	"github.com/golang-jwt/jwt/v4"
)

// ValidateJWTClaims requires a valid parsed token and validates configured
// OIDC issuer and audience requirements.
// Empty expected values preserve the legacy behavior of signature and time validation only.
func ValidateJWTClaims(token *jwt.Token, expectedIssuer, expectedAudience string) error {
	if token == nil {
		return fmt.Errorf("token is nil")
	}
	if !token.Valid {
		return fmt.Errorf("token is invalid")
	}
	if expectedIssuer == "" && expectedAudience == "" {
		return nil
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return fmt.Errorf("token claims are not map claims")
	}

	if expectedIssuer != "" && !claims.VerifyIssuer(expectedIssuer, true) {
		return fmt.Errorf("token issuer does not match the expected issuer")
	}

	if expectedAudience != "" && !claims.VerifyAudience(expectedAudience, true) {
		return fmt.Errorf("token audience does not include the expected audience")
	}

	return nil
}
