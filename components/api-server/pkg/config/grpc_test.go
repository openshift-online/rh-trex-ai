package config

import (
	"errors"
	"strings"
	"testing"
)

func TestGRPCConfigValidate(t *testing.T) {
	tests := []struct {
		name      string
		cfg       GRPCConfig
		wantField string
	}{
		{name: "tls disabled", cfg: GRPCConfig{}},
		{name: "tls disabled ignores files", cfg: GRPCConfig{TLSCertFile: "tls.crt"}},
		{name: "tls enabled with both files", cfg: GRPCConfig{EnableTLS: true, TLSCertFile: "tls.crt", TLSKeyFile: "tls.key"}},
		{name: "tls enabled without cert", cfg: GRPCConfig{EnableTLS: true, TLSKeyFile: "tls.key"}, wantField: "grpc-tls-cert-file"},
		{name: "tls enabled without key", cfg: GRPCConfig{EnableTLS: true, TLSCertFile: "tls.crt"}, wantField: "grpc-tls-key-file"},
		{name: "tls enabled without files", cfg: GRPCConfig{EnableTLS: true}, wantField: "grpc-tls-cert-file/grpc-tls-key-file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("Validate() unexpected error: %v", err)
				}
				return
			}
			var validationErr *ConfigValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("Validate() error = %v, want *ConfigValidationError", err)
			}
			if validationErr.Field != tt.wantField {
				t.Fatalf("Validate() field = %q, want %q", validationErr.Field, tt.wantField)
			}
			if !strings.Contains(err.Error(), "--grpc-enable-tls") {
				t.Fatalf("Validate() error %q does not name --grpc-enable-tls", err.Error())
			}
		})
	}
}

func TestTLSConfigMinTLSVersion(t *testing.T) {
	tests := map[string]uint16{"1.2": 0x0303, "1.3": 0x0304, "1.1": 0, "": 0}
	for version, want := range tests {
		c := &TLSConfig{MinVersion: version}
		if got := c.MinTLSVersion(); got != want {
			t.Errorf("MinTLSVersion(%q) = %#x, want %#x", version, got, want)
		}
	}
}
