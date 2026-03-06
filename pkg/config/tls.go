package config

import (
	"crypto/tls"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
	
	tlsutil "github.com/openshift-online/rh-trex-ai/pkg/tls"
)

// TLSConfig holds TLS configuration for servers and clients
type TLSConfig struct {
	// Server TLS Configuration
	EnableTLS           bool     `json:"enable_tls"`
	CertFile            string   `json:"cert_file"`
	KeyFile             string   `json:"key_file"`
	CAFile              string   `json:"ca_file"`
	
	// TLS Security Settings
	MinVersion          string   `json:"min_version"`          // TLS version: "1.2", "1.3"
	MaxVersion          string   `json:"max_version"`          // TLS version: "1.2", "1.3"
	CipherSuites        []string `json:"cipher_suites"`        // Allowed cipher suites
	InsecureSkipVerify  bool     `json:"insecure_skip_verify"` // Skip certificate verification (dev only)
	
	// Mutual TLS
	EnableClientAuth    bool     `json:"enable_client_auth"`   // Require client certificates
	ClientCAFile        string   `json:"client_ca_file"`       // CA for validating client certs
	
	// Kubernetes Integration
	AutoDetectKubernetes bool    `json:"auto_detect_kubernetes"` // Auto-configure for Kubernetes
	CustomCAFiles        []string `json:"custom_ca_files"`        // Additional CA files to load
	
	// Advanced Settings
	ServerName          string   `json:"server_name"`          // Expected server name for clients
	EnableSNI           bool     `json:"enable_sni"`           // Server Name Indication support
	PreferServerCiphers bool     `json:"prefer_server_ciphers"` // Prefer server cipher suite order
}

// NewTLSConfig creates a new TLS configuration with secure defaults
func NewTLSConfig() *TLSConfig {
	return &TLSConfig{
		// Disabled by default, must be explicitly enabled
		EnableTLS:            false,
		CertFile:             "",
		KeyFile:              "",
		CAFile:               "",
		
		// Secure defaults
		MinVersion:           "1.2",  // TLS 1.2 minimum for compatibility
		MaxVersion:           "1.3",  // TLS 1.3 maximum for future-proofing
		CipherSuites:         []string{}, // Empty means use Go defaults (which are secure)
		InsecureSkipVerify:   false,
		
		// mTLS disabled by default
		EnableClientAuth:     false,
		ClientCAFile:         "",
		
		// Kubernetes auto-detection enabled
		AutoDetectKubernetes: true,
		CustomCAFiles:        []string{},
		
		// Sensible defaults
		ServerName:           "",
		EnableSNI:            true,
		PreferServerCiphers:  true,
	}
}

// AddFlags adds TLS configuration flags to the provided flag set
func (c *TLSConfig) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" && !strings.HasSuffix(prefix, "-") {
		prefix += "-"
	}
	
	// Basic TLS settings
	fs.BoolVar(&c.EnableTLS, prefix+"enable-tls", c.EnableTLS, "Enable TLS encryption")
	fs.StringVar(&c.CertFile, prefix+"tls-cert-file", c.CertFile, "TLS certificate file path")
	fs.StringVar(&c.KeyFile, prefix+"tls-key-file", c.KeyFile, "TLS private key file path")
	fs.StringVar(&c.CAFile, prefix+"tls-ca-file", c.CAFile, "TLS CA certificate file path")
	
	// Security settings
	fs.StringVar(&c.MinVersion, prefix+"tls-min-version", c.MinVersion, "Minimum TLS version (1.2, 1.3)")
	fs.StringVar(&c.MaxVersion, prefix+"tls-max-version", c.MaxVersion, "Maximum TLS version (1.2, 1.3)")
	fs.StringSliceVar(&c.CipherSuites, prefix+"tls-cipher-suites", c.CipherSuites, "Allowed TLS cipher suites")
	fs.BoolVar(&c.InsecureSkipVerify, prefix+"tls-insecure-skip-verify", c.InsecureSkipVerify, "Skip TLS certificate verification (development only)")
	
	// Mutual TLS
	fs.BoolVar(&c.EnableClientAuth, prefix+"tls-enable-client-auth", c.EnableClientAuth, "Require client certificates for mutual TLS")
	fs.StringVar(&c.ClientCAFile, prefix+"tls-client-ca-file", c.ClientCAFile, "CA file for validating client certificates")
	
	// Kubernetes integration
	fs.BoolVar(&c.AutoDetectKubernetes, prefix+"tls-auto-detect-kubernetes", c.AutoDetectKubernetes, "Auto-configure TLS for Kubernetes/OpenShift")
	fs.StringSliceVar(&c.CustomCAFiles, prefix+"tls-custom-ca-files", c.CustomCAFiles, "Additional CA files to load")
	
	// Advanced settings
	fs.StringVar(&c.ServerName, prefix+"tls-server-name", c.ServerName, "Expected server name for TLS verification")
	fs.BoolVar(&c.EnableSNI, prefix+"tls-enable-sni", c.EnableSNI, "Enable Server Name Indication")
	fs.BoolVar(&c.PreferServerCiphers, prefix+"tls-prefer-server-ciphers", c.PreferServerCiphers, "Prefer server cipher suite order")
}

// ReadFiles reads TLS configuration from files and environment variables
func (c *TLSConfig) ReadFiles() error {
	// Check for certificate files from environment if not set
	if c.CertFile == "" {
		if certFile := os.Getenv("TLS_CERT_FILE"); certFile != "" {
			c.CertFile = certFile
		}
	}
	
	if c.KeyFile == "" {
		if keyFile := os.Getenv("TLS_KEY_FILE"); keyFile != "" {
			c.KeyFile = keyFile
		}
	}
	
	if c.CAFile == "" {
		if caFile := os.Getenv("TLS_CA_FILE"); caFile != "" {
			c.CAFile = caFile
		}
	}
	
	return nil
}

// Validate checks the TLS configuration for consistency and security
func (c *TLSConfig) Validate() error {
	if !c.EnableTLS {
		return nil // TLS disabled, nothing to validate
	}
	
	// Check for required certificate files
	if c.CertFile == "" || c.KeyFile == "" {
		return &ConfigValidationError{
			Field:   "tls-cert-file/tls-key-file",
			Message: "TLS certificate and key files are required when TLS is enabled",
		}
	}
	
	// Validate TLS versions
	if err := c.validateTLSVersion(c.MinVersion, "min-version"); err != nil {
		return err
	}
	if err := c.validateTLSVersion(c.MaxVersion, "max-version"); err != nil {
		return err
	}
	
	// Check version ordering
	minVer := c.parseVersionToInt(c.MinVersion)
	maxVer := c.parseVersionToInt(c.MaxVersion)
	if minVer > maxVer {
		return &ConfigValidationError{
			Field:   "tls-min-version/tls-max-version",
			Message: "minimum TLS version cannot be higher than maximum version",
		}
	}
	
	// Security check: warn about insecure settings
	if c.InsecureSkipVerify {
		return &ConfigValidationError{
			Field:   "tls-insecure-skip-verify",
			Message: "insecure TLS verification should only be used in development",
		}
	}
	
	// Validate client auth settings
	if c.EnableClientAuth && c.ClientCAFile == "" {
		return &ConfigValidationError{
			Field:   "tls-client-ca-file",
			Message: "client CA file is required when client authentication is enabled",
		}
	}
	
	return nil
}

// BuildServerTLSConfig creates a server TLS configuration
func (c *TLSConfig) BuildServerTLSConfig() (*tls.Config, error) {
	if !c.EnableTLS {
		return nil, nil // TLS disabled
	}
	
	// Auto-detect Kubernetes environment
	if c.AutoDetectKubernetes {
		if config, err := tlsutil.AutoConfigureTLS(c.ServerName); err == nil {
			return c.applySecuritySettings(config), nil
		}
		// Fall through to manual configuration if auto-detection fails
	}
	
	// Manual TLS configuration
	var config *tls.Config
	var err error
	
	if c.EnableClientAuth && c.ClientCAFile != "" {
		// Mutual TLS
		config, err = tlsutil.NewMutualTLSConfig(c.CertFile, c.KeyFile, c.ClientCAFile)
	} else {
		// Server TLS only
		config, err = tlsutil.NewServerTLSConfig(c.CertFile, c.KeyFile)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS config: %w", err)
	}
	
	return c.applySecuritySettings(config), nil
}

// BuildClientTLSConfig creates a client TLS configuration
func (c *TLSConfig) BuildClientTLSConfig() (*tls.Config, error) {
	if !c.EnableTLS {
		return nil, nil // TLS disabled
	}
	
	// Auto-detect Kubernetes environment
	if c.AutoDetectKubernetes {
		if config, err := tlsutil.NewKubernetesClientTLSConfig(c.ServerName, false); err == nil {
			return c.applySecuritySettings(config), nil
		}
		// Fall through to manual configuration if auto-detection fails
	}
	
	// Manual client TLS configuration
	config, err := tlsutil.NewClientTLSConfig(c.ServerName, c.CAFile, c.InsecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("failed to create client TLS config: %w", err)
	}
	
	return c.applySecuritySettings(config), nil
}

// applySecuritySettings applies configured security settings to a TLS config
func (c *TLSConfig) applySecuritySettings(config *tls.Config) *tls.Config {
	// Apply version settings
	if minVer := c.tlsVersionToInt(c.MinVersion); minVer != 0 {
		config.MinVersion = minVer
	}
	if maxVer := c.tlsVersionToInt(c.MaxVersion); maxVer != 0 {
		config.MaxVersion = maxVer
	}
	
	// Apply cipher suites if specified
	if len(c.CipherSuites) > 0 {
		suites := make([]uint16, 0, len(c.CipherSuites))
		for _, suite := range c.CipherSuites {
			if suiteID := c.cipherSuiteByName(suite); suiteID != 0 {
				suites = append(suites, suiteID)
			}
		}
		if len(suites) > 0 {
			config.CipherSuites = suites
		}
	}
	
	// Apply other settings
	config.InsecureSkipVerify = c.InsecureSkipVerify
	config.PreferServerCipherSuites = c.PreferServerCiphers
	config.ServerName = c.ServerName
	
	return config
}

// validateTLSVersion validates a TLS version string
func (c *TLSConfig) validateTLSVersion(version, field string) error {
	switch version {
	case "1.2", "1.3":
		return nil
	case "1.0", "1.1":
		return &ConfigValidationError{
			Field:   "tls-" + field,
			Message: fmt.Sprintf("TLS %s is not secure - use 1.2 or 1.3", version),
		}
	default:
		return &ConfigValidationError{
			Field:   "tls-" + field,
			Message: fmt.Sprintf("invalid TLS version '%s' - supported: 1.2, 1.3", version),
		}
	}
}

// parseVersionToInt converts version string to int for comparison
func (c *TLSConfig) parseVersionToInt(version string) int {
	switch version {
	case "1.2":
		return 12
	case "1.3":
		return 13
	default:
		return 0
	}
}

// tlsVersionToInt converts version string to tls constant
func (c *TLSConfig) tlsVersionToInt(version string) uint16 {
	switch version {
	case "1.2":
		return tls.VersionTLS12
	case "1.3":
		return tls.VersionTLS13
	default:
		return 0
	}
}

// cipherSuiteByName returns cipher suite ID by name
func (c *TLSConfig) cipherSuiteByName(name string) uint16 {
	// Map common cipher suite names to IDs
	suites := map[string]uint16{
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":       tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":       tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384":     tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256":     tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305":        tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305":      tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
	}
	
	return suites[name]
}

// GetSecurityInfo returns information about TLS security settings
func (c *TLSConfig) GetSecurityInfo() map[string]interface{} {
	info := make(map[string]interface{})
	
	info["tls_enabled"] = c.EnableTLS
	info["min_version"] = c.MinVersion
	info["max_version"] = c.MaxVersion
	info["client_auth_enabled"] = c.EnableClientAuth
	info["kubernetes_auto_detect"] = c.AutoDetectKubernetes
	info["insecure_skip_verify"] = c.InsecureSkipVerify
	
	if c.EnableTLS {
		info["cert_file_configured"] = c.CertFile != ""
		info["ca_file_configured"] = c.CAFile != ""
		info["custom_cipher_suites"] = len(c.CipherSuites) > 0
	}
	
	// Include Kubernetes environment info if auto-detection is enabled
	if c.AutoDetectKubernetes {
		info["kubernetes_environment"] = tlsutil.GetTLSEnvironmentInfo()
	}
	
	return info
}