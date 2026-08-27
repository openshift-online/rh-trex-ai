package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootHelpExposesIntegratedTUICommand(t *testing.T) {
	root := newRootCommand()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	help := output.String()
	if !strings.Contains(help, "tui") || !strings.Contains(help, "Browse and operate the service from a terminal UI") {
		t.Fatalf("root help omitted integrated TUI command:\n%s", help)
	}
}
