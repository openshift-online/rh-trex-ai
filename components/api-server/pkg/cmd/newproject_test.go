package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToSnake(t *testing.T) {
	tests := map[string]string{
		"my-service": "my_service",
		"my_service": "my_service",
		"MyService":  "my_service",
		"my-Service": "my_service",
		"svc":        "svc",
	}
	for in, want := range tests {
		if got := toSnake(in); got != want {
			t.Errorf("toSnake(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateProjectName(t *testing.T) {
	for _, name := range []string{"my-service", "svc", "my_svc2"} {
		if err := validateProjectName(name); err != nil {
			t.Errorf("validateProjectName(%q) = %v, want nil", name, err)
		}
	}
	for _, name := range []string{"", "1svc", "my svc", "a/b", "my.svc", "-svc"} {
		if err := validateProjectName(name); err == nil {
			t.Errorf("validateProjectName(%q) = nil, want error", name)
		}
	}
}

func TestScaffoldProjectHyphenatedName(t *testing.T) {
	dir := t.TempDir()
	if err := scaffoldProject("my-svc", "github.com/acme", dir); err != nil {
		t.Fatalf("scaffoldProject: %v", err)
	}
	root := filepath.Join(dir, "my-svc")

	// Files starting with "." must be copied (go:embed all: prefix).
	for _, rel := range []string{
		".claude/commands/trex.kind.new.md",
		"data/generated/tui/.trex-tui-generated",
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("scaffold is missing %s: %v", rel, err)
		}
	}

	// The protobuf package must not contain '-'.
	proto, err := os.ReadFile(filepath.Join(root, "proto", "my_svc", "v1", "common.proto"))
	if err != nil {
		t.Fatalf("read common.proto: %v", err)
	}
	if !strings.Contains(string(proto), "package my_svc.v1;") {
		t.Errorf("common.proto has the wrong package:\n%s", proto)
	}

	// The spec must be compiled in, not read from the source tree at runtime.
	embed, err := os.ReadFile(filepath.Join(root, "pkg", "api", "openapi_embed.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(embed), "//go:embed openapi/api/openapi.yaml") || strings.Contains(string(embed), "runtime.Caller") {
		t.Errorf("openapi_embed.go does not embed the spec:\n%s", embed)
	}
	if strings.HasPrefix(string(embed), "//go:build ignore") {
		t.Error("openapi_embed.go still has the //go:build ignore header")
	}
}
