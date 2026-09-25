package server

// Transport security for the gRPC listener.
//
// The shared TLS configuration (--enable-tls) covers every listener at once. The
// gRPC flags (--grpc-enable-tls, --grpc-tls-cert-file, --grpc-tls-key-file) let an
// operator enable TLS on the gRPC listener alone, which is the common layout when
// REST is edge-terminated by a Route or Ingress but gRPC needs end-to-end TLS.
// When both are enabled the shared configuration wins, so a single shared
// certificate keeps working. The gRPC-only key pair is re-read whenever either
// file changes on disk, so certificate rotation (cert-manager, OpenShift service
// CA) is picked up by new connections without a restart.

import (
	"crypto/tls"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang/glog"
	"google.golang.org/grpc/credentials"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
)

// grpcTransportCredentials returns the credentials the gRPC server listens with,
// or nil when neither the shared nor the gRPC-only TLS configuration is enabled.
func grpcTransportCredentials(shared *config.TLSConfig, grpcCfg *config.GRPCConfig) (credentials.TransportCredentials, error) {
	if shared != nil && shared.EnableTLS {
		tlsConfig, err := shared.BuildServerTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("build shared TLS configuration for gRPC (--enable-tls): %w", err)
		}
		glog.Infof("Using enhanced TLS configuration with minimum version %s", tlsVersionName(tlsConfig.MinVersion))
		return credentials.NewTLS(tlsConfig), nil
	}

	if grpcCfg == nil || !grpcCfg.EnableTLS {
		return nil, nil
	}

	tlsConfig, err := newGRPCOnlyTLSConfig(shared, grpcCfg)
	if err != nil {
		return nil, err
	}
	glog.Infof("Using gRPC TLS configuration from %s with minimum version %s; the key pair is reloaded when the files change",
		grpcCfg.TLSCertFile, tlsVersionName(tlsConfig.MinVersion))
	return credentials.NewTLS(tlsConfig), nil
}

// newGRPCOnlyTLSConfig builds the server TLS configuration for --grpc-enable-tls.
// It requires TLS 1.2 (or the shared --tls-min-version when that is higher) and
// offers only HTTP/2 over ALPN, which is what gRPC clients negotiate.
func newGRPCOnlyTLSConfig(shared *config.TLSConfig, grpcCfg *config.GRPCConfig) (*tls.Config, error) {
	if err := grpcCfg.Validate(); err != nil {
		return nil, err
	}
	reloader, err := newKeyPairReloader(grpcCfg.TLSCertFile, grpcCfg.TLSKeyFile)
	if err != nil {
		return nil, err
	}

	minVersion := uint16(tls.VersionTLS12)
	if shared != nil {
		if v := shared.MinTLSVersion(); v > minVersion {
			minVersion = v
		}
	}

	return &tls.Config{
		GetCertificate: reloader.getCertificate,
		MinVersion:     minVersion,
		NextProtos:     []string{"h2"},
	}, nil
}

func tlsVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return "unknown"
	}
}

type fileStamp struct {
	modTime time.Time
	size    int64
}

func statFile(path string) (fileStamp, error) {
	info, err := os.Stat(path)
	if err != nil {
		return fileStamp{}, fmt.Errorf("stat %s: %w", path, err)
	}
	return fileStamp{modTime: info.ModTime(), size: info.Size()}, nil
}

// keyPairReloader serves a certificate and key loaded from disk, reloading them
// on the next handshake after either file's modification time or size changes.
type keyPairReloader struct {
	certFile string
	keyFile  string
	// errorf reports a failed reload; tests replace it to observe failures.
	errorf func(format string, args ...interface{})

	mu       sync.Mutex
	cert     *tls.Certificate
	certSeen fileStamp
	keySeen  fileStamp
}

// newKeyPairReloader loads the key pair once so that a missing or malformed
// file is reported at startup rather than on the first handshake.
func newKeyPairReloader(certFile, keyFile string) (*keyPairReloader, error) {
	r := &keyPairReloader{certFile: certFile, keyFile: keyFile, errorf: glog.Errorf}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

// load reads both files and replaces the served key pair. The caller holds r.mu
// or is the constructor.
func (r *keyPairReloader) load() error {
	certStamp, err := statFile(r.certFile)
	if err != nil {
		return fmt.Errorf("load gRPC TLS key pair: %w", err)
	}
	keyStamp, err := statFile(r.keyFile)
	if err != nil {
		return fmt.Errorf("load gRPC TLS key pair: %w", err)
	}
	cert, err := tls.LoadX509KeyPair(r.certFile, r.keyFile)
	if err != nil {
		return fmt.Errorf("load gRPC TLS key pair (%s, %s): %w", r.certFile, r.keyFile, err)
	}
	r.cert = &cert
	r.certSeen = certStamp
	r.keySeen = keyStamp
	return nil
}

// changed reports whether either file differs from the last successful load.
// A file that cannot be stat'ed (for example mid-rotation) counts as unchanged.
func (r *keyPairReloader) changed() bool {
	certStamp, err := statFile(r.certFile)
	if err != nil {
		return false
	}
	keyStamp, err := statFile(r.keyFile)
	if err != nil {
		return false
	}
	return certStamp != r.certSeen || keyStamp != r.keySeen
}

// getCertificate is the tls.Config.GetCertificate hook. A reload that fails
// keeps serving the previous key pair so that a half-written renewal cannot take
// the listener down; it is retried on the next handshake.
func (r *keyPairReloader) getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.changed() {
		if err := r.load(); err != nil {
			r.errorf("gRPC TLS key pair changed on disk but could not be reloaded; serving the previous one: %v", err)
		} else {
			glog.Infof("gRPC TLS key pair reloaded from %s", r.certFile)
		}
	}
	return r.cert, nil
}
