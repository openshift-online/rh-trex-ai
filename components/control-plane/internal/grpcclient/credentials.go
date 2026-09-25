// Package grpcclient builds the transport credentials the controller uses to
// dial the API server's gRPC listener.
package grpcclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/openshift-online/rh-trex-ai/components/control-plane/internal/config"
)

// TransportCredentials returns plaintext credentials unless cfg.GRPCTLS is set.
// With TLS the server certificate is verified against cfg.GRPCTLSCAFile when
// set, or the system roots otherwise, and cfg.GRPCTLSServerName, when set,
// overrides the name used for SNI and verification.
func TransportCredentials(cfg *config.Config) (credentials.TransportCredentials, error) {
	if !cfg.GRPCTLS {
		return insecure.NewCredentials(), nil
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: cfg.GRPCTLSServerName,
	}
	if cfg.GRPCTLSCAFile != "" {
		pem, err := os.ReadFile(cfg.GRPCTLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("reading TREX_GRPC_TLS_CA_FILE %s: %w", cfg.GRPCTLSCAFile, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("parsing TREX_GRPC_TLS_CA_FILE %s: no PEM certificates found", cfg.GRPCTLSCAFile)
		}
		tlsConfig.RootCAs = pool
	}
	return credentials.NewTLS(tlsConfig), nil
}
