package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestGenerateEntityVerifyAndRegenerateRequireManifest(t *testing.T) {
	dir := t.TempDir()
	for _, flag := range []string{"--verify", "--regenerate"} {
		cmd := NewGenerateEntityCommand()
		cmd.SetArgs([]string{flag, "--dir", dir, "--run-make=false"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "requires --manifest") {
			t.Errorf("%s without --manifest = %v, want a 'requires --manifest' error", flag, err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("command wrote %d entries into %s, want none", len(entries), dir)
	}
}
