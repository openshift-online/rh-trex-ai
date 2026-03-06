package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"strings"
)

// SecureTLSConfig returns a TLS configuration with security hardening
// Enforces TLS 1.2+ and secure cipher suites for production environments
func SecureTLSConfig() *tls.Config {
	return &tls.Config{
		// Enforce TLS 1.2 minimum (1.3 preferred, 1.2 for compatibility)
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS13,
		
		// Use only secure cipher suites (Go's defaults are good but we're explicit)
		CipherSuites: []uint16{
			// TLS 1.3 cipher suites (handled automatically by Go)
			// TLS 1.2 cipher suites - prefer AEAD and ECDHE
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		},
		
		// Prefer server cipher suite order
		PreferServerCipherSuites: true,
		
		// Session tickets are OK with TLS 1.2+ but not configurable in this field
		
		// Use only secure curves
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
		},
		
		// Require certificates
		ClientAuth: tls.NoClientCert, // Can be overridden for mTLS
	}
}

// NewServerTLSConfig creates a server TLS configuration from cert/key files
func NewServerTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	if certFile == "" || keyFile == "" {
		return nil, fmt.Errorf("both cert-file and key-file must be specified")
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS key pair: %w", err)
	}

	config := SecureTLSConfig()
	config.Certificates = []tls.Certificate{cert}
	
	return config, nil
}

// NewClientTLSConfig creates a client TLS configuration with optional custom CA
func NewClientTLSConfig(serverName string, caFile string, insecureSkipVerify bool) (*tls.Config, error) {
	config := SecureTLSConfig()
	config.ServerName = serverName
	config.InsecureSkipVerify = insecureSkipVerify
	
	if caFile != "" {
		caCert, err := ioutil.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		
		config.RootCAs = caCertPool
	}
	
	return config, nil
}

// NewMutualTLSConfig creates a mutual TLS configuration for client authentication
func NewMutualTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	config, err := NewServerTLSConfig(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	
	if caFile != "" {
		caCert, err := ioutil.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		
		config.ClientCAs = caCertPool
		config.ClientAuth = tls.RequireAndVerifyClientCert
	}
	
	return config, nil
}

// ValidateTLSConfig performs security validation on a TLS configuration
func ValidateTLSConfig(config *tls.Config) []string {
	var warnings []string
	
	// Check minimum TLS version
	if config.MinVersion < tls.VersionTLS12 {
		warnings = append(warnings, "TLS version below 1.2 is not secure")
	}
	
	// Check for insecure settings
	if config.InsecureSkipVerify {
		warnings = append(warnings, "InsecureSkipVerify=true disables certificate verification")
	}
	
	// Check cipher suites for weak algorithms
	for _, suite := range config.CipherSuites {
		switch suite {
		case tls.TLS_RSA_WITH_RC4_128_SHA,
			 tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
			 tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
			 tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA:
			warnings = append(warnings, fmt.Sprintf("Weak cipher suite detected: %x", suite))
		}
	}
	
	return warnings
}

// GetTLSVersionString returns human-readable TLS version
func GetTLSVersionString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (%x)", version)
	}
}

// PrintTLSInfo logs TLS configuration information for debugging
func PrintTLSInfo(config *tls.Config, prefix string) []string {
	var info []string
	
	info = append(info, fmt.Sprintf("%sMin TLS Version: %s", prefix, GetTLSVersionString(config.MinVersion)))
	info = append(info, fmt.Sprintf("%sMax TLS Version: %s", prefix, GetTLSVersionString(config.MaxVersion)))
	info = append(info, fmt.Sprintf("%sCipher Suites: %d configured", prefix, len(config.CipherSuites)))
	info = append(info, fmt.Sprintf("%sClient Auth: %v", prefix, config.ClientAuth))
	
	if len(config.Certificates) > 0 {
		info = append(info, fmt.Sprintf("%sCertificates: %d loaded", prefix, len(config.Certificates)))
	}
	
	// Check for security warnings
	warnings := ValidateTLSConfig(config)
	if len(warnings) > 0 {
		info = append(info, fmt.Sprintf("%sSecurity warnings: %s", prefix, strings.Join(warnings, ", ")))
	}
	
	return info
}