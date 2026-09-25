package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	GRPCServerAddr string
	APIServerURL   string
	Namespace      string
	LogLevel       string

	// GRPCTLS dials the gRPC server with TLS instead of plaintext (TREX_GRPC_TLS).
	GRPCTLS bool
	// GRPCTLSCAFile is a PEM CA bundle used to verify the gRPC server
	// certificate; the system roots are used when empty (TREX_GRPC_TLS_CA_FILE).
	GRPCTLSCAFile string
	// GRPCTLSServerName overrides the name used for SNI and certificate
	// verification; the host of GRPCServerAddr is used when empty
	// (TREX_GRPC_TLS_SERVER_NAME).
	GRPCTLSServerName string
}

func Load() (*Config, error) {
	cfg := &Config{
		GRPCServerAddr:    getEnv("TREX_GRPC_SERVER_ADDR", "localhost:9000"),
		APIServerURL:      getEnv("TREX_API_SERVER_URL", "http://localhost:8000"),
		Namespace:         getEnv("TREX_NAMESPACE", "rh-trex"),
		LogLevel:          strings.ToLower(getEnv("TREX_LOG_LEVEL", "info")),
		GRPCTLSCAFile:     getEnv("TREX_GRPC_TLS_CA_FILE", ""),
		GRPCTLSServerName: getEnv("TREX_GRPC_TLS_SERVER_NAME", ""),
	}

	if cfg.GRPCServerAddr == "" {
		return nil, fmt.Errorf("TREX_GRPC_SERVER_ADDR is required")
	}

	grpcTLS, err := strconv.ParseBool(getEnv("TREX_GRPC_TLS", "false"))
	if err != nil {
		return nil, fmt.Errorf("parsing TREX_GRPC_TLS: %w", err)
	}
	cfg.GRPCTLS = grpcTLS

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
