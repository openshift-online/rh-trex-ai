package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuditConfigDefaults(t *testing.T) {
	c := NewAuditConfig()
	if !c.Enabled || c.Reads || c.Sink != AuditSinkStdout || c.SealInterval != time.Second || c.HMACKeyFile != "" {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestAuditConfigValidate(t *testing.T) {
	for _, tc := range []struct {
		sink     string
		interval time.Duration
		ok       bool
	}{
		{"stdout", time.Second, true},
		{"none", time.Second, true},
		{"file:/tmp/audit.jsonl", time.Second, true},
		{"file:", time.Second, false},
		{"syslog", time.Second, false},
		{"stdout", 10 * time.Millisecond, false},
	} {
		c := NewAuditConfig()
		c.Sink, c.SealInterval = tc.sink, tc.interval
		if err := c.Validate(); (err == nil) != tc.ok {
			t.Errorf("sink %q interval %s: err=%v, want ok=%v", tc.sink, tc.interval, err, tc.ok)
		}
	}
}

func TestAuditConfigHMACKey(t *testing.T) {
	dir := t.TempDir()
	short := filepath.Join(dir, "short")
	good := filepath.Join(dir, "good")
	if err := os.WriteFile(short, []byte(strings.Repeat("k", 16)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(good, []byte(strings.Repeat("k", 32)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := NewAuditConfig()
	c.HMACKeyFile = short
	if err := c.ReadFiles(); err == nil || !strings.Contains(err.Error(), "--audit-hmac-key-file") {
		t.Fatalf("short key must fail naming the flag, got %v", err)
	}
	c.HMACKeyFile = filepath.Join(dir, "missing")
	if err := c.ReadFiles(); err == nil {
		t.Fatal("missing key file must fail")
	}
	c.HMACKeyFile = good
	if err := c.ReadFiles(); err != nil {
		t.Fatal(err)
	}
	if len(c.HMACKey()) != 32 {
		t.Fatalf("trailing newline must be trimmed, got %d bytes", len(c.HMACKey()))
	}
	c.HMACKeyFile = ""
	if err := c.ReadFiles(); err != nil || c.HMACKey() != nil {
		t.Fatal("no key file means no key")
	}
}
