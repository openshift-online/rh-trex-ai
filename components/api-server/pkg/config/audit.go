package config

import (
	"fmt"
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
	Enabled bool   `json:"enabled"`
	Reads   bool   `json:"reads"`
	Sink    string `json:"sink"`
}

func NewAuditConfig() *AuditConfig {
	return &AuditConfig{Enabled: true, Sink: AuditSinkStdout}
}

func (c *AuditConfig) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&c.Enabled, "enable-audit", c.Enabled, "Capture audit events")
	fs.BoolVar(&c.Reads, "audit-reads", c.Reads, "Record allowed get, list, and watch requests on every kind, not only kinds registered with audit.RecordReads")
	fs.StringVar(&c.Sink, "audit-sink", c.Sink, "Destination of the default audit recorder: stdout or file:<path>")
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
	return nil
}
