package config

import (
	"os"
	"strings"

	"github.com/spf13/pflag"
)

// AuthConfig holds authentication configuration for both JWT and Bearer token auth
type AuthConfig struct {
	// JWT Authentication (existing)
	EnableJWT     bool   `json:"enable_jwt"`
	EnableAuthz   bool   `json:"enable_authz"`
	JwkCertURL    string `json:"jwk_cert_url"`
	JwkCertFile   string `json:"jwk_cert_file"`
	
	// Bearer Token Authentication (new)
	EnableBearer  bool     `json:"enable_bearer"`
	BearerToken   string   `json:"-"` // Don't serialize token to JSON
	BypassPaths   []string `json:"bypass_paths"`
	BypassMethods []string `json:"bypass_methods"`
}

// NewAuthConfig creates a new AuthConfig with default values
func NewAuthConfig() *AuthConfig {
	return &AuthConfig{
		// JWT defaults (preserve existing behavior from ServerConfig)
		EnableJWT:   true,
		EnableAuthz: true,
		JwkCertURL:  "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/certs",
		JwkCertFile: "",
		
		// Bearer token defaults (disabled by default)
		EnableBearer:  false,
		BearerToken:   "",
		BypassPaths:   []string{"/healthcheck", "/metrics", "/api/rh-trex/v1/openapi", "/openapi"},
		BypassMethods: []string{"/grpc.health.v1.Health/", "/grpc.reflection.v1alpha.ServerReflection/"},
	}
}

// AddFlags adds authentication configuration flags to the provided flag set
func (c *AuthConfig) AddFlags(fs *pflag.FlagSet) {
	// JWT flags (existing)
	fs.BoolVar(&c.EnableJWT, "enable-jwt", c.EnableJWT, "Enable JWT authentication validation")
	fs.BoolVar(&c.EnableAuthz, "enable-authz", c.EnableAuthz, "Enable authorization on endpoints")
	fs.StringVar(&c.JwkCertURL, "jwk-cert-url", c.JwkCertURL, "JWK Certificate URL for JWT validation")
	fs.StringVar(&c.JwkCertFile, "jwk-cert-file", c.JwkCertFile, "Local JWK Certificate file")
	
	// Bearer token flags (new)
	fs.BoolVar(&c.EnableBearer, "enable-bearer", c.EnableBearer, "Enable bearer token authentication")
	fs.StringVar(&c.BearerToken, "bearer-token", c.BearerToken, "Expected bearer token for authentication")
	fs.StringSliceVar(&c.BypassPaths, "auth-bypass-paths", c.BypassPaths, "HTTP paths that bypass authentication")
	fs.StringSliceVar(&c.BypassMethods, "auth-bypass-methods", c.BypassMethods, "gRPC methods that bypass authentication")
}

// ReadFiles reads authentication configuration from files and environment variables
func (c *AuthConfig) ReadFiles() error {
	// Read JWK cert file if specified
	if c.JwkCertFile != "" {
		// JWK cert file reading is handled by the JWT middleware
		// No action needed here
	}
	
	// Read bearer token from environment if not set via flag
	if c.BearerToken == "" {
		if token := os.Getenv("API_TOKEN"); token != "" {
			c.BearerToken = token
		} else if token := os.Getenv("BEARER_TOKEN"); token != "" {
			c.BearerToken = token
		}
	}
	
	return nil
}

// Validate checks the authentication configuration for consistency
func (c *AuthConfig) Validate() error {
	// If both JWT and Bearer are disabled, that might be intentional for development
	// but we should log a warning (handled by the caller)
	
	// If Bearer auth is enabled, we need a token
	if c.EnableBearer && c.BearerToken == "" {
		return &ConfigValidationError{
			Field:   "bearer-token",
			Message: "bearer token is required when --enable-bearer=true, set via --bearer-token flag or API_TOKEN environment variable",
		}
	}
	
	// Ensure bypass paths start with /
	for i, path := range c.BypassPaths {
		if !strings.HasPrefix(path, "/") {
			c.BypassPaths[i] = "/" + path
		}
	}
	
	return nil
}

// IsAuthEnabled returns true if any authentication method is enabled
func (c *AuthConfig) IsAuthEnabled() bool {
	return c.EnableJWT || c.EnableBearer
}

// ConfigValidationError represents a configuration validation error
type ConfigValidationError struct {
	Field   string
	Message string
}

func (e *ConfigValidationError) Error() string {
	return "configuration error for " + e.Field + ": " + e.Message
}