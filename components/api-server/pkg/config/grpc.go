package config

import (
	"strings"

	"github.com/spf13/pflag"
)

type GRPCConfig struct {
	EnableGRPC  bool   `json:"enable_grpc"`
	BindAddress string `json:"bind_address"`
	EnableTLS   bool   `json:"enable_tls"`
	TLSCertFile string `json:"tls_cert_file"`
	TLSKeyFile  string `json:"tls_key_file"`
}

func NewGRPCConfig() *GRPCConfig {
	return &GRPCConfig{
		EnableGRPC:  true,
		BindAddress: "localhost:9000",
	}
}

func (c *GRPCConfig) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&c.EnableGRPC, "enable-grpc", c.EnableGRPC, "Enable gRPC server")
	fs.StringVar(&c.BindAddress, "grpc-server-bindaddress", c.BindAddress, "gRPC server bind address")
	fs.BoolVar(&c.EnableTLS, "grpc-enable-tls", c.EnableTLS, "Enable TLS for gRPC server")
	fs.StringVar(&c.TLSCertFile, "grpc-tls-cert-file", c.TLSCertFile, "gRPC TLS certificate file")
	fs.StringVar(&c.TLSKeyFile, "grpc-tls-key-file", c.TLSKeyFile, "gRPC TLS key file")
}

func (c *GRPCConfig) ReadFiles() error {
	return nil
}

// Validate rejects --grpc-enable-tls without both --grpc-tls-cert-file and
// --grpc-tls-key-file. It only checks that the flags are set; whether the files
// exist and parse is checked when the gRPC server loads them.
func (c *GRPCConfig) Validate() error {
	if !c.EnableTLS {
		return nil
	}
	var missing []string
	if c.TLSCertFile == "" {
		missing = append(missing, "grpc-tls-cert-file")
	}
	if c.TLSKeyFile == "" {
		missing = append(missing, "grpc-tls-key-file")
	}
	if len(missing) > 0 {
		return &ConfigValidationError{
			Field:   strings.Join(missing, "/"),
			Message: "gRPC TLS certificate and key files are required when --grpc-enable-tls is set",
		}
	}
	return nil
}
