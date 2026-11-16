package config

import (
	"fmt"
	"net"
	"strings"

	"github.com/spf13/pflag"
)

// Audit sinks accepted by --audit-sink for the default recorder.
const (
	AuditSinkStdout = "stdout"
	// AuditSinkFilePrefix selects a file: "file:/var/log/audit.jsonl".
	AuditSinkFilePrefix = "file:"
)

// AuditConfig configures audit capture (SEC-004).
type AuditConfig struct {
	Enabled        bool     `json:"enabled"`
	Reads          bool     `json:"reads"`
	Sink           string   `json:"sink"`
	TrustedProxies []string `json:"trusted_proxies"`
}

func NewAuditConfig() *AuditConfig {
	return &AuditConfig{Enabled: true, Sink: AuditSinkStdout}
}

func (c *AuditConfig) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&c.Enabled, "enable-audit", c.Enabled, "Capture audit events")
	fs.BoolVar(&c.Reads, "audit-reads", c.Reads, "Record allowed get, list, and watch requests on every kind, not only kinds registered with audit.RecordReads")
	fs.StringVar(&c.Sink, "audit-sink", c.Sink, "Destination of the default audit recorder: stdout or file:<path>")
	fs.StringSliceVar(&c.TrustedProxies, "audit-trusted-proxies", c.TrustedProxies, "CIDRs or IPs of proxies whose X-Forwarded-For is trusted when recording the client IP (empty: never trust it)")
}

// TrustedProxyNets parses TrustedProxies. A bare IP is a single-address network.
func (c *AuditConfig) TrustedProxyNets() ([]*net.IPNet, error) {
	var nets []*net.IPNet
	for _, entry := range c.TrustedProxies {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if !strings.Contains(entry, "/") {
			ip := net.ParseIP(entry)
			if ip == nil {
				return nil, fmt.Errorf("invalid IP %q", entry)
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			entry = fmt.Sprintf("%s/%d", ip.String(), bits)
		}
		_, n, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q", entry)
		}
		nets = append(nets, n)
	}
	return nets, nil
}

func (c *AuditConfig) ReadFiles() error { return nil }

// SinkFile returns the path of a file sink and whether the sink is a file.
func (c *AuditConfig) SinkFile() (string, bool) {
	if strings.HasPrefix(c.Sink, AuditSinkFilePrefix) {
		return strings.TrimPrefix(c.Sink, AuditSinkFilePrefix), true
	}
	return "", false
}

func (c *AuditConfig) Validate() error {
	switch {
	case c.Sink == AuditSinkStdout:
	case strings.HasPrefix(c.Sink, AuditSinkFilePrefix):
		if path, _ := c.SinkFile(); path == "" {
			return &ConfigValidationError{Field: "audit-sink", Message: "file sink needs a path, e.g. file:/var/log/audit.jsonl"}
		}
	default:
		return &ConfigValidationError{Field: "audit-sink", Message: fmt.Sprintf("unknown sink %q; use stdout or file:<path>", c.Sink)}
	}
	if _, err := c.TrustedProxyNets(); err != nil {
		return &ConfigValidationError{Field: "audit-trusted-proxies", Message: err.Error()}
	}
	return nil
}
