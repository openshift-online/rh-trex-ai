package main

import (
	"bytes"
	"testing"
)

// The TUI is no longer integrated into the server binary; it lives in the
// separate trexctl component (CLI + TUI). The server root must not expose it.
func TestRootHelpOmitsTUICommand(t *testing.T) {
	root := newRootCommand()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	help := output.String()
	if bytes.Contains(output.Bytes(), []byte("tui")) {
		t.Fatalf("server root help unexpectedly exposes the integrated TUI command (moved to trexctl):\n%s", help)
	}
}
