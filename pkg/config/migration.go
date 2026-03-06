package config

// MigrateServerConfigToAuthConfig migrates authentication settings from ServerConfig to AuthConfig
// This ensures backward compatibility while moving to the new authentication structure
func MigrateServerConfigToAuthConfig(serverConfig *ServerConfig, authConfig *AuthConfig) {
	// Only migrate if the auth config hasn't been explicitly configured
	// (i.e., it's still at default values)
	
	// EnableJWT migration removed - ServerConfig.EnableJWT no longer exists
	
	if serverConfig.EnableAuthz != authConfig.EnableAuthz {
		authConfig.EnableAuthz = serverConfig.EnableAuthz
	}
	
	if serverConfig.JwkCertURL != "" && serverConfig.JwkCertURL != authConfig.JwkCertURL {
		authConfig.JwkCertURL = serverConfig.JwkCertURL
	}
	
	if serverConfig.JwkCertFile != "" {
		authConfig.JwkCertFile = serverConfig.JwkCertFile
	}
}

// GetEffectiveAuthConfig returns the effective authentication configuration
// after applying any necessary migrations from ServerConfig
func (c *ApplicationConfig) GetEffectiveAuthConfig() *AuthConfig {
	// Create a copy of the auth config to avoid modifying the original
	effectiveAuth := &AuthConfig{
		EnableJWT:     c.Auth.EnableJWT,
		EnableAuthz:   c.Auth.EnableAuthz,
		JwkCertURL:    c.Auth.JwkCertURL,
		JwkCertFile:   c.Auth.JwkCertFile,
		EnableBearer:  c.Auth.EnableBearer,
		BearerToken:   c.Auth.BearerToken,
		BypassPaths:   c.Auth.BypassPaths,
		BypassMethods: c.Auth.BypassMethods,
	}
	
	// Apply migration from ServerConfig for backward compatibility
	MigrateServerConfigToAuthConfig(c.Server, effectiveAuth)
	
	return effectiveAuth
}