package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	GRPCServerAddr string
	APIServerURL   string
	Namespace      string
	LogLevel       string
}

func Load() (*Config, error) {
	cfg := &Config{
		GRPCServerAddr: getEnv("TREX_GRPC_SERVER_ADDR", "localhost:9000"),
		APIServerURL:   getEnv("TREX_API_SERVER_URL", "http://localhost:8000"),
		Namespace:      getEnv("TREX_NAMESPACE", "rh-trex"),
		LogLevel:       strings.ToLower(getEnv("TREX_LOG_LEVEL", "info")),
	}

	if cfg.GRPCServerAddr == "" {
		return nil, fmt.Errorf("TREX_GRPC_SERVER_ADDR is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
