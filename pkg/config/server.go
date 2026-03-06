package config

import (
	"time"

	"github.com/spf13/pflag"
)

type ServerConfig struct {
	Hostname           string        `json:"hostname"`
	BindAddress        string        `json:"bind_address"`
	ReadTimeout        time.Duration `json:"read_timeout"`
	WriteTimeout       time.Duration `json:"write_timeout"`
	HTTPSCertFile      string        `json:"https_cert_file"`
	HTTPSKeyFile       string        `json:"https_key_file"`
	EnableHTTPS        bool          `json:"enable_https"`
	EnableAuthz        bool          `json:"enable_authz"`
	JwkCertFile        string        `json:"jwk_cert_file"`
	JwkCertURL         string        `json:"jwk_cert_url"`
	ACLFile            string        `json:"acl_file"`
	CORSAllowedOrigins []string      `json:"cors_allowed_origins"`
	CORSAllowedHeaders []string      `json:"cors_allowed_headers"`
}

func NewServerConfig() *ServerConfig {
	return &ServerConfig{
		Hostname:      "",
		BindAddress:   "localhost:8000",
		ReadTimeout:   5 * time.Second,
		WriteTimeout:  30 * time.Second,
		EnableHTTPS:   false,
		EnableAuthz:   true,
		JwkCertFile:   "",
		JwkCertURL:    "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/certs", // Default to Red Hat SSO, configurable for other OIDC providers
		ACLFile:       "",
		HTTPSCertFile: "",
		HTTPSKeyFile:  "",
	}
}

func (s *ServerConfig) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.BindAddress, "api-server-bindaddress", s.BindAddress, "API server bind adddress")
	fs.StringVar(&s.Hostname, "api-server-hostname", s.Hostname, "Server's public hostname")
	fs.DurationVar(&s.ReadTimeout, "http-read-timeout", s.ReadTimeout, "HTTP server read timeout")
	fs.DurationVar(&s.WriteTimeout, "http-write-timeout", s.WriteTimeout, "HTTP server write timeout")
	fs.StringVar(&s.HTTPSCertFile, "https-cert-file", s.HTTPSCertFile, "The path to the tls.crt file.")
	fs.StringVar(&s.HTTPSKeyFile, "https-key-file", s.HTTPSKeyFile, "The path to the tls.key file.")
	fs.BoolVar(&s.EnableHTTPS, "enable-https", s.EnableHTTPS, "Enable HTTPS rather than HTTP")
	// JWT authentication flags have been moved to AuthConfig
	// Legacy fields maintained for backward compatibility but flags handled by AuthConfig
	fs.StringVar(&s.ACLFile, "acl-file", s.ACLFile, "Access control list file")
	fs.StringSliceVar(&s.CORSAllowedOrigins, "cors-allowed-origins", s.CORSAllowedOrigins, "Comma-separated list of CORS allowed origins")
	fs.StringSliceVar(&s.CORSAllowedHeaders, "cors-allowed-headers", s.CORSAllowedHeaders, "Comma-separated list of additional CORS allowed headers")
}

func (s *ServerConfig) ReadFiles() error {
	return nil
}
