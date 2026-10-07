package cmd

import (
	"testing"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/tui"
)

type staticProvider struct{}

func (staticProvider) GetToken() (string, error) { return "token", nil }

// TestTUICommandReExportsKeepExistingCallersWorking compiles and exercises the
// compatibility surface; the behavior is tested in pkg/tuicmd.
func TestTUICommandReExportsKeepExistingCallersWorking(t *testing.T) {
	var provider tui.TokenProvider = staticProvider{}
	command := NewTUICommand(
		func() ([]byte, error) { return nil, nil },
		WithTokenProvider(provider),
		WithSession(func() (Session, error) { return Session{TokenProvider: provider}, nil }),
	)
	if command.Use != "tui" {
		t.Fatalf("command = %q, want tui", command.Use)
	}
	for _, flag := range []string{"server", "token-file", "insecure", "trust-origin", "refresh-interval"} {
		if command.Flags().Lookup(flag) == nil {
			t.Fatalf("flag --%s missing from the re-exported command", flag)
		}
	}
}
