package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

// Audit sink kinds accepted by --audit-sink.
const (
	AuditSinkStdout = "stdout"
	AuditSinkNone   = "none"
	// AuditSinkFilePrefix selects a file sink: "file:/var/log/audit.jsonl".
	AuditSinkFilePrefix = "file:"

	// AuditMinHMACKeyBytes is the shortest accepted HMAC key.
	AuditMinHMACKeyBytes = 32
	// AuditMinSealInterval is the shortest accepted --audit-seal-interval.
	AuditMinSealInterval = 100 * time.Millisecond
)

// AuditConfig configures audit recording, sealing, and export (SEC-004).
type AuditConfig struct {
	Enabled      bool          `json:"enabled"`
	Reads        bool          `json:"reads"`
	Sink         string        `json:"sink"`
	HMACKeyFile  string        `json:"hmac_key_file"`
	SealInterval time.Duration `json:"seal_interval"`

	// hmacKey is loaded from HMACKeyFile by ReadFiles. It is never serialized.
	hmacKey []byte
}

func NewAuditConfig() *AuditConfig {
	return &AuditConfig{
		Enabled:      true,
		Reads:        false,
		Sink:         AuditSinkStdout,
		SealInterval: time.Second,
	}
}

func (c *AuditConfig) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&c.Enabled, "enable-audit", c.Enabled, "Record, seal, and export audit events")
	fs.BoolVar(&c.Reads, "audit-reads", c.Reads, "Also record allowed get, list, and watch requests")
	fs.StringVar(&c.Sink, "audit-sink", c.Sink, "Audit export sink: stdout, file:<path>, or none")
	fs.StringVar(&c.HMACKeyFile, "audit-hmac-key-file", c.HMACKeyFile, "File holding the audit hash-chain HMAC key (at least 32 bytes)")
	fs.DurationVar(&c.SealInterval, "audit-seal-interval", c.SealInterval, "Time between audit sealing passes")
}

// ReadFiles loads the HMAC key. A configured but unreadable or short key is an error.
func (c *AuditConfig) ReadFiles() error {
	c.hmacKey = nil
	if c.HMACKeyFile == "" {
		return nil
	}
	contents, err := ReadFile(c.HMACKeyFile)
	if err != nil {
		return fmt.Errorf("--audit-hmac-key-file: %w", err)
	}
	key := []byte(strings.TrimRight(contents, " \t\r\n"))
	if len(key) < AuditMinHMACKeyBytes {
		return fmt.Errorf("--audit-hmac-key-file: key is %d bytes, need at least %d", len(key), AuditMinHMACKeyBytes)
	}
	c.hmacKey = key
	return nil
}

// HMACKey returns the loaded key, or nil when none is configured.
func (c *AuditConfig) HMACKey() []byte {
	return c.hmacKey
}

// SetHMACKey sets the key directly. It is meant for tests.
func (c *AuditConfig) SetHMACKey(key []byte) {
	c.hmacKey = key
}

// SinkFile returns the path of a file sink and whether the sink is a file.
func (c *AuditConfig) SinkFile() (string, bool) {
	if strings.HasPrefix(c.Sink, AuditSinkFilePrefix) {
		return strings.TrimPrefix(c.Sink, AuditSinkFilePrefix), true
	}
	return "", false
}

func (c *AuditConfig) Validate() error {
	switch {
	case c.Sink == AuditSinkStdout, c.Sink == AuditSinkNone:
	case strings.HasPrefix(c.Sink, AuditSinkFilePrefix):
		if path, _ := c.SinkFile(); path == "" {
			return &ConfigValidationError{Field: "audit-sink", Message: "file sink needs a path, e.g. file:/var/log/audit.jsonl"}
		}
	default:
		return &ConfigValidationError{Field: "audit-sink", Message: fmt.Sprintf("unknown sink %q; use stdout, file:<path>, or none", c.Sink)}
	}
	if c.SealInterval < AuditMinSealInterval {
		return &ConfigValidationError{Field: "audit-seal-interval", Message: fmt.Sprintf("must be at least %s", AuditMinSealInterval)}
	}
	return nil
}
