package config

import "testing"

func TestAuditConfigDefaults(t *testing.T) {
	c := NewAuditConfig()
	if !c.Enabled || c.Reads || c.Sink != AuditSinkStdout {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestAuditConfigValidate(t *testing.T) {
	for _, tc := range []struct {
		sink string
		ok   bool
	}{
		{"stdout", true},
		{"file:/tmp/audit.jsonl", true},
		{"file:", false},
		{"none", false},
		{"syslog", false},
	} {
		c := NewAuditConfig()
		c.Sink = tc.sink
		if err := c.Validate(); (err == nil) != tc.ok {
			t.Errorf("sink %q: err=%v, want ok=%v", tc.sink, err, tc.ok)
		}
	}
	c := NewAuditConfig()
	if path, ok := c.SinkFile(); ok || path != "" {
		t.Fatal("stdout is not a file sink")
	}
	c.Sink = "file:/x"
	if path, ok := c.SinkFile(); !ok || path != "/x" {
		t.Fatalf("got %q %v", path, ok)
	}
}
