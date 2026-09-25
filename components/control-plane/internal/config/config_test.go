package config

import (
	"strings"
	"testing"
)

func TestLoadGRPCTLS(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    bool
		wantErr bool
	}{
		{name: "unset", value: "", want: false},
		{name: "true", value: "true", want: true},
		{name: "one", value: "1", want: true},
		{name: "false", value: "false", want: false},
		{name: "garbage", value: "yes please", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TREX_GRPC_TLS", tt.value)
			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() with TREX_GRPC_TLS=%q succeeded, want an error", tt.value)
				}
				if !strings.Contains(err.Error(), "TREX_GRPC_TLS") {
					t.Fatalf("Load() error %q does not name TREX_GRPC_TLS", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if cfg.GRPCTLS != tt.want {
				t.Fatalf("GRPCTLS = %v, want %v", cfg.GRPCTLS, tt.want)
			}
		})
	}
}

func TestLoadGRPCTLSFiles(t *testing.T) {
	t.Setenv("TREX_GRPC_TLS_CA_FILE", "/etc/trex/ca.crt")
	t.Setenv("TREX_GRPC_TLS_SERVER_NAME", "trex-grpc.example.test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.GRPCTLSCAFile != "/etc/trex/ca.crt" {
		t.Fatalf("GRPCTLSCAFile = %q, want /etc/trex/ca.crt", cfg.GRPCTLSCAFile)
	}
	if cfg.GRPCTLSServerName != "trex-grpc.example.test" {
		t.Fatalf("GRPCTLSServerName = %q, want trex-grpc.example.test", cfg.GRPCTLSServerName)
	}
}
