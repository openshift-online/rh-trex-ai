package config

import (
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
