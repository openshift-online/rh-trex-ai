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

func TestAuditTrustedProxies(t *testing.T) {
	c := NewAuditConfig()
	c.TrustedProxies = []string{"10.128.0.0/14", "192.168.1.10", "fd00::/8", "::1"}
	nets, err := c.TrustedProxyNets()
	if err != nil || len(nets) != 4 {
		t.Fatalf("got %v, %v", nets, err)
	}
	if nets[1].String() != "192.168.1.10/32" || nets[3].String() != "::1/128" {
		t.Fatalf("bare IPs must become single-address networks: %v", nets)
	}
	for _, bad := range []string{"10.0.0.0/33", "not-an-ip", "10.0.0"} {
		c.TrustedProxies = []string{bad}
		if err := c.Validate(); err == nil {
			t.Fatalf("%q must be rejected", bad)
		}
	}
}
