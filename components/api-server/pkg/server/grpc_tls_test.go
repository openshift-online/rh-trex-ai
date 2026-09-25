package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/peer"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
)

const testGRPCServerName = "grpc.example.test"

// testKeyPair is a CA-signed leaf written to disk, plus a pool trusting its CA.
type testKeyPair struct {
	pool     *x509.CertPool
	certFile string
	keyFile  string
	serial   *big.Int
}

// writeTestKeyPair writes a fresh CA-signed ECDSA leaf into dir as tls.crt and
// tls.key. Each call uses a new CA and the given leaf serial, so two key pairs
// are distinguishable by the leaf the server presents.
func writeTestKeyPair(t *testing.T, dir string, serial int64) testKeyPair {
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
		SerialNumber:          big.NewInt(serial * 1000),
		Subject:               pkix.Name{CommonName: fmt.Sprintf("test ca %d", serial)},
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
		DNSNames:     []string{testGRPCServerName},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: testGRPCServerName},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create leaf certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(leafKey)
	if err != nil {
		t.Fatalf("marshal leaf key: %v", err)
	}

	certFile := filepath.Join(dir, "tls.crt")
	keyFile := filepath.Join(dir, "tls.key")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}), 0o600); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	return testKeyPair{pool: pool, certFile: certFile, keyFile: keyFile, serial: big.NewInt(serial)}
}

// bumpModTime moves the files' mtimes forward so a rewrite is visible even on
// filesystems with coarse (one second) timestamps.
func bumpModTime(t *testing.T, paths ...string) {
	t.Helper()
	future := time.Now().Add(2 * time.Second)
	for _, path := range paths {
		if err := os.Chtimes(path, future, future); err != nil {
			t.Fatalf("chtimes %s: %v", path, err)
		}
	}
}

// startTLSHealthServer serves the gRPC health service with creds on a loopback
// port and returns its address.
func startTLSHealthServer(t *testing.T, creds credentials.TransportCredentials) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer(grpc.Creds(creds))
	healthgrpc.RegisterHealthServer(srv, health.NewServer())
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(srv.GracefulStop)
	return listener.Addr().String()
}

// checkHealth opens a new connection with creds, calls Health/Check, and returns
// the TLS state the client observed.
func checkHealth(t *testing.T, addr string, creds credentials.TransportCredentials) (tls.ConnectionState, error) {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var p peer.Peer
	if _, err := healthgrpc.NewHealthClient(conn).Check(ctx, &healthgrpc.HealthCheckRequest{}, grpc.Peer(&p)); err != nil {
		return tls.ConnectionState{}, err
	}
	info, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return tls.ConnectionState{}, fmt.Errorf("peer auth info is %T, want credentials.TLSInfo", p.AuthInfo)
	}
	return info.State, nil
}

func clientTLS(pool *x509.CertPool) credentials.TransportCredentials {
	return credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"h2"},
		RootCAs:    pool,
		ServerName: testGRPCServerName,
	})
}

func leafSerial(t *testing.T, state tls.ConnectionState) *big.Int {
	t.Helper()
	if len(state.PeerCertificates) == 0 {
		t.Fatal("server presented no certificate")
	}
	return state.PeerCertificates[0].SerialNumber
}

func grpcOnlyConfig(pair testKeyPair) (*config.TLSConfig, *config.GRPCConfig) {
	return config.NewTLSConfig(), &config.GRPCConfig{EnableTLS: true, TLSCertFile: pair.certFile, TLSKeyFile: pair.keyFile}
}

func TestGRPCTransportCredentialsDisabled(t *testing.T) {
	creds, err := grpcTransportCredentials(config.NewTLSConfig(), config.NewGRPCConfig())
	if err != nil {
		t.Fatalf("grpcTransportCredentials() unexpected error: %v", err)
	}
	if creds != nil {
		t.Fatalf("grpcTransportCredentials() = %v, want nil when no TLS is enabled", creds)
	}
}

func TestGRPCOnlyTLSServesHealthOverH2(t *testing.T) {
	pair := writeTestKeyPair(t, t.TempDir(), 1)
	shared, grpcCfg := grpcOnlyConfig(pair)
	creds, err := grpcTransportCredentials(shared, grpcCfg)
	if err != nil {
		t.Fatalf("grpcTransportCredentials() unexpected error: %v", err)
	}
	if creds == nil {
		t.Fatal("grpcTransportCredentials() returned nil credentials with --grpc-enable-tls set")
	}
	addr := startTLSHealthServer(t, creds)

	state, err := checkHealth(t, addr, clientTLS(pair.pool))
	if err != nil {
		t.Fatalf("Health/Check over TLS failed: %v", err)
	}
	if state.NegotiatedProtocol != "h2" {
		t.Fatalf("negotiated ALPN %q, want h2", state.NegotiatedProtocol)
	}
	if state.Version < tls.VersionTLS12 {
		t.Fatalf("negotiated TLS version %#x, want at least TLS 1.2", state.Version)
	}
	if got := leafSerial(t, state); got.Cmp(pair.serial) != 0 {
		t.Fatalf("server presented leaf serial %v, want %v", got, pair.serial)
	}

	if _, err := checkHealth(t, addr, insecure.NewCredentials()); err == nil {
		t.Fatal("plaintext Health/Check against the TLS listener succeeded")
	}
}

func TestGRPCOnlyTLSConfig(t *testing.T) {
	pair := writeTestKeyPair(t, t.TempDir(), 2)
	shared, grpcCfg := grpcOnlyConfig(pair)

	cfg, err := newGRPCOnlyTLSConfig(shared, grpcCfg)
	if err != nil {
		t.Fatalf("newGRPCOnlyTLSConfig() unexpected error: %v", err)
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %#x, want TLS 1.2", cfg.MinVersion)
	}
	if len(cfg.NextProtos) != 1 || cfg.NextProtos[0] != "h2" {
		t.Fatalf("NextProtos = %v, want [h2]", cfg.NextProtos)
	}

	shared.MinVersion = "1.3"
	cfg, err = newGRPCOnlyTLSConfig(shared, grpcCfg)
	if err != nil {
		t.Fatalf("newGRPCOnlyTLSConfig() unexpected error: %v", err)
	}
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Fatalf("MinVersion = %#x, want TLS 1.3 from --tls-min-version", cfg.MinVersion)
	}
}

func TestSharedTLSTakesPrecedenceForGRPC(t *testing.T) {
	sharedPair := writeTestKeyPair(t, t.TempDir(), 10)
	grpcPair := writeTestKeyPair(t, t.TempDir(), 20)

	shared := config.NewTLSConfig()
	shared.EnableTLS = true
	shared.CertFile = sharedPair.certFile
	shared.KeyFile = sharedPair.keyFile
	// Kubernetes auto-detection would replace the configured files outside a
	// cluster; use the files explicitly.
	shared.AutoDetectKubernetes = false
	grpcCfg := &config.GRPCConfig{EnableTLS: true, TLSCertFile: grpcPair.certFile, TLSKeyFile: grpcPair.keyFile}

	creds, err := grpcTransportCredentials(shared, grpcCfg)
	if err != nil {
		t.Fatalf("grpcTransportCredentials() unexpected error: %v", err)
	}
	addr := startTLSHealthServer(t, creds)

	state, err := checkHealth(t, addr, clientTLS(sharedPair.pool))
	if err != nil {
		t.Fatalf("Health/Check trusting the shared certificate failed: %v", err)
	}
	if got := leafSerial(t, state); got.Cmp(sharedPair.serial) != 0 {
		t.Fatalf("server presented leaf serial %v, want the shared certificate %v", got, sharedPair.serial)
	}
}

func TestGRPCOnlyTLSFailsFastOnBadFiles(t *testing.T) {
	dir := t.TempDir()
	pair := writeTestKeyPair(t, dir, 30)
	garbage := filepath.Join(dir, "garbage.crt")
	if err := os.WriteFile(garbage, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("write garbage: %v", err)
	}
	missing := filepath.Join(dir, "missing.crt")

	tests := []struct {
		name     string
		certFile string
		keyFile  string
		wantText string
	}{
		{name: "unset cert", keyFile: pair.keyFile, wantText: "grpc-tls-cert-file"},
		{name: "unset key", certFile: pair.certFile, wantText: "grpc-tls-key-file"},
		{name: "missing cert", certFile: missing, keyFile: pair.keyFile, wantText: missing},
		{name: "garbage cert", certFile: garbage, keyFile: pair.keyFile, wantText: garbage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grpcCfg := &config.GRPCConfig{EnableTLS: true, TLSCertFile: tt.certFile, TLSKeyFile: tt.keyFile}
			creds, err := grpcTransportCredentials(config.NewTLSConfig(), grpcCfg)
			if err == nil {
				t.Fatalf("grpcTransportCredentials() = %v, want an error", creds)
			}
			if !strings.Contains(err.Error(), tt.wantText) {
				t.Fatalf("error %q does not mention %q", err.Error(), tt.wantText)
			}
		})
	}
}

func TestGRPCOnlyTLSReloadsRenewedKeyPair(t *testing.T) {
	dir := t.TempDir()
	first := writeTestKeyPair(t, dir, 40)
	shared, grpcCfg := grpcOnlyConfig(first)
	creds, err := grpcTransportCredentials(shared, grpcCfg)
	if err != nil {
		t.Fatalf("grpcTransportCredentials() unexpected error: %v", err)
	}
	addr := startTLSHealthServer(t, creds)

	state, err := checkHealth(t, addr, clientTLS(first.pool))
	if err != nil {
		t.Fatalf("Health/Check before renewal failed: %v", err)
	}
	if got := leafSerial(t, state); got.Cmp(first.serial) != 0 {
		t.Fatalf("before renewal: leaf serial %v, want %v", got, first.serial)
	}

	// Renew in place: same paths, new CA and leaf.
	renewed := writeTestKeyPair(t, dir, 50)
	bumpModTime(t, renewed.certFile, renewed.keyFile)

	state, err = checkHealth(t, addr, clientTLS(renewed.pool))
	if err != nil {
		t.Fatalf("Health/Check trusting the renewed certificate failed, so it was not reloaded: %v", err)
	}
	if got := leafSerial(t, state); got.Cmp(renewed.serial) != 0 {
		t.Fatalf("after renewal: leaf serial %v, want %v", got, renewed.serial)
	}
}

func TestGRPCOnlyTLSKeepsServingWhenRenewalIsUnreadable(t *testing.T) {
	dir := t.TempDir()
	pair := writeTestKeyPair(t, dir, 60)

	reloader, err := newKeyPairReloader(pair.certFile, pair.keyFile)
	if err != nil {
		t.Fatalf("newKeyPairReloader() unexpected error: %v", err)
	}
	var mu sync.Mutex
	var reloadErrors []string
	reloader.errorf = func(format string, args ...interface{}) {
		mu.Lock()
		defer mu.Unlock()
		reloadErrors = append(reloadErrors, fmt.Sprintf(format, args...))
	}
	addr := startTLSHealthServer(t, credentials.NewTLS(&tls.Config{
		GetCertificate: reloader.getCertificate,
		MinVersion:     tls.VersionTLS12,
		NextProtos:     []string{"h2"},
	}))

	if err := os.WriteFile(pair.certFile, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("overwrite certificate: %v", err)
	}
	bumpModTime(t, pair.certFile)

	state, err := checkHealth(t, addr, clientTLS(pair.pool))
	if err != nil {
		t.Fatalf("listener stopped serving the previous certificate after a bad renewal: %v", err)
	}
	if got := leafSerial(t, state); got.Cmp(pair.serial) != 0 {
		t.Fatalf("leaf serial %v, want the previous certificate %v", got, pair.serial)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(reloadErrors) == 0 {
		t.Fatal("failed reload was not reported")
	}
	if !strings.Contains(reloadErrors[0], pair.certFile) {
		t.Fatalf("reload error %q does not name %s", reloadErrors[0], pair.certFile)
	}
}

func TestKeyPairReloaderRejectsMissingFiles(t *testing.T) {
	_, err := newKeyPairReloader(filepath.Join(t.TempDir(), "missing.crt"), "missing.key")
	if err == nil {
		t.Fatal("newKeyPairReloader() with missing files succeeded")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("newKeyPairReloader() error = %v, want it to wrap os.ErrNotExist", err)
	}
}
