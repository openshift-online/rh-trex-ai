package config

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// Old command lines that still pass the removed API client flags must parse.
func TestRemovedFlagsStillParse(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	NewApplicationConfig().AddFlags(fs)
	args := []string{
		"--enable-mock=false",
		"--debug",
		"--api-client-id-file=/secrets/service/api-service.clientId",
		"--api-client-secret-file=/secrets/service/api-service.clientSecret",
		"--self-token-file=x",
		"--api-base-url=https://example.test",
		"--api-token-url=https://example.test/token",
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse old flags: %v", err)
	}
	for _, f := range removedFlags {
		flag := fs.Lookup(f.name)
		if flag == nil || flag.Deprecated == "" || !flag.Hidden {
			t.Errorf("flag %q: want registered, hidden and deprecated", f.name)
		}
	}
}

// The production environment once crash-looped when these files were missing.
func TestReadFilesNoAPIClientSecrets(t *testing.T) {
	SetProjectRootDir(t.TempDir())
	t.Cleanup(func() { SetProjectRootDir("") })
	c := NewApplicationConfig()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	c.AddFlags(fs)
	if err := fs.Parse([]string{"--enable-mock=false"}); err != nil {
		t.Fatal(err)
	}
	for _, msg := range c.ReadFiles() {
		if strings.Contains(msg, "api-service") || strings.HasPrefix(msg, "APIClient ") {
			t.Errorf("unexpected API client file read: %s", msg)
		}
	}
}
