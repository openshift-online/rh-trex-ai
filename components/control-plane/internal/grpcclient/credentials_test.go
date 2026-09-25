package grpcclient

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/openshift-online/rh-trex-ai/components/control-plane/internal/config"
)

const testServerName = "grpc.example.test"

// writeTestPKI writes a CA certificate to dir/ca.crt and returns it with a
// server certificate for testServerName signed by that CA.
func writeTestPKI(t *testing.T, dir string) (string, tls.Certificate) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	caTemplate := &x509.Certificate{
		BasicConstraintsValid: true,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test ca"},
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA certificate: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse CA certificate: %v", err)
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	leafTemplate := &x509.Certificate{
		DNSNames:     []string{testServerName},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: testServerName},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create leaf certificate: %v", err)
	}

	caFile := filepath.Join(dir, "ca.crt")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0o600); err != nil {
		t.Fatalf("write CA file: %v", err)
	}
	return caFile, tls.Certificate{Certificate: [][]byte{leafDER}, PrivateKey: leafKey}
}

// startTLSHealthServer serves the gRPC health service over TLS on a loopback
// port and returns its address.
func startTLSHealthServer(t *testing.T, cert tls.Certificate) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})))
	healthgrpc.RegisterHealthServer(srv, health.NewServer())
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(srv.GracefulStop)
	return listener.Addr().String()
}

func checkHealth(t *testing.T, addr string, creds credentials.TransportCredentials) error {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer func() { _ = conn.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = healthgrpc.NewHealthClient(conn).Check(ctx, &healthgrpc.HealthCheckRequest{})
	return err
}

func TestTransportCredentialsPlaintextByDefault(t *testing.T) {
	creds, err := TransportCredentials(&config.Config{})
	if err != nil {
		t.Fatalf("TransportCredentials() unexpected error: %v", err)
	}
	if got := creds.Info().SecurityProtocol; got != "insecure" {
		t.Fatalf("SecurityProtocol = %q, want insecure", got)
	}
}

func TestTransportCredentialsTLS(t *testing.T) {
	caFile, _ := writeTestPKI(t, t.TempDir())

	creds, err := TransportCredentials(&config.Config{GRPCTLS: true, GRPCTLSCAFile: caFile})
	if err != nil {
		t.Fatalf("TransportCredentials() unexpected error: %v", err)
	}
	if got := creds.Info().SecurityProtocol; got != "tls" {
		t.Fatalf("SecurityProtocol = %q, want tls", got)
	}

	// No CA file: the system roots are used.
	if _, err := TransportCredentials(&config.Config{GRPCTLS: true}); err != nil {
		t.Fatalf("TransportCredentials() with system roots unexpected error: %v", err)
	}
}

func TestTransportCredentialsRejectsBadCAFile(t *testing.T) {
	dir := t.TempDir()
	garbage := filepath.Join(dir, "garbage.crt")
	if err := os.WriteFile(garbage, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("write garbage: %v", err)
	}

	for name, caFile := range map[string]string{
		"missing": filepath.Join(dir, "missing.crt"),
		"garbage": garbage,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := TransportCredentials(&config.Config{GRPCTLS: true, GRPCTLSCAFile: caFile}); err == nil {
				t.Fatalf("TransportCredentials() with a %s CA file succeeded", name)
			}
		})
	}
}

func TestTransportCredentialsDialsTLSServer(t *testing.T) {
	caFile, serverCert := writeTestPKI(t, t.TempDir())
	addr := startTLSHealthServer(t, serverCert)

	trusted, err := TransportCredentials(&config.Config{GRPCTLS: true, GRPCTLSCAFile: caFile, GRPCTLSServerName: testServerName})
	if err != nil {
		t.Fatalf("TransportCredentials() unexpected error: %v", err)
	}
	if err := checkHealth(t, addr, trusted); err != nil {
		t.Fatalf("Health/Check with the CA file failed: %v", err)
	}

	systemRoots, err := TransportCredentials(&config.Config{GRPCTLS: true, GRPCTLSServerName: testServerName})
	if err != nil {
		t.Fatalf("TransportCredentials() unexpected error: %v", err)
	}
	if err := checkHealth(t, addr, systemRoots); err == nil {
		t.Fatal("Health/Check verifying against the system roots accepted a test-CA certificate")
	}

	wrongName, err := TransportCredentials(&config.Config{GRPCTLS: true, GRPCTLSCAFile: caFile, GRPCTLSServerName: "other.example.test"})
	if err != nil {
		t.Fatalf("TransportCredentials() unexpected error: %v", err)
	}
	if err := checkHealth(t, addr, wrongName); err == nil {
		t.Fatal("Health/Check with a mismatched server name override succeeded")
	}

	plaintext, err := TransportCredentials(&config.Config{})
	if err != nil {
		t.Fatalf("TransportCredentials() unexpected error: %v", err)
	}
	if err := checkHealth(t, addr, plaintext); err == nil {
		t.Fatal("plaintext Health/Check against the TLS server succeeded")
	}
}
