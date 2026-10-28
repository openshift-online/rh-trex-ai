package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openshift-online/rh-trex-ai/components/api-server/templates"
)

func TestPluralize(t *testing.T) {
	tests := []struct {
		word string
		want string
	}{
		{"Dinosaur", "Dinosaurs"},
		{"Category", "Categories"},
		{"Registry", "Registries"},
		{"Policy", "Policies"},
		{"Company", "Companies"},
	}
	for _, tt := range tests {
		if got := pluralize(tt.word, ""); got != tt.want {
			t.Errorf("pluralize(%q) = %q, want %q", tt.word, got, tt.want)
		}
	}
}

func TestPluralizeExplicit(t *testing.T) {
	if got := pluralize("Mouse", "Mice"); got != "Mice" {
		t.Errorf("pluralize with explicit plural = %q, want %q", got, "Mice")
	}
}

func TestParseFields(t *testing.T) {
	fields, err := parseFields("name:string,count:int:required,active:bool:optional")
	if err != nil {
		t.Fatalf("parseFields failed: %v", err)
	}
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}

	name := fields[0]
	if name.Name != "Name" || name.GoType != "*string" || name.Nullable != true {
		t.Errorf("unexpected name field: %+v", name)
	}

	count := fields[1]
	if count.Name != "Count" || count.GoType != "int" || count.Nullable != false {
		t.Errorf("unexpected count field: %+v", count)
	}

	active := fields[2]
	if active.Name != "Active" || active.GoType != "*bool" || active.Nullable != true {
		t.Errorf("unexpected active field: %+v", active)
	}
}

func TestParseFieldsInvalid(t *testing.T) {
	if _, err := parseFields("name"); err == nil {
		t.Error("expected error for invalid field format")
	}
	if _, err := parseFields("name:somethingelse"); err == nil {
		t.Error("expected error for unsupported type")
	}
	if _, err := parseFields("name:string:bogus"); err == nil {
		t.Error("expected error for invalid modifier")
	}
}

func TestRunGeneratesEntity(t *testing.T) {
	base := t.TempDir()

	cmdDir := filepath.Join(base, "cmd", "trex")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("mkdir cmd: %v", err)
	}
	mainGo := `package main

import (
	"github.com/spf13/cobra"
)

func main() {}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	openAPIDir := filepath.Join(base, "openapi")
	if err := os.MkdirAll(openAPIDir, 0755); err != nil {
		t.Fatalf("mkdir openapi: %v", err)
	}
	openapiYAML := "paths:\n  # AUTO-ADD NEW PATHS\ncomponents:\n  schemas:\n    # AUTO-ADD NEW SCHEMAS\n"
	if err := os.WriteFile(filepath.Join(openAPIDir, "openapi.yaml"), []byte(openapiYAML), 0644); err != nil {
		t.Fatalf("write openapi.yaml: %v", err)
	}

	err := Run(Options{
		TemplatesFS: templates.GeneratorFS,
		BaseDir:     base,
		Kind:        "Gizmo",
		Repo:        "github.com/openshift-online",
		Project:     "rh-trex-ai",
		Library:     "github.com/openshift-online/rh-trex-ai/components/api-server",
		Fields:      "name:string,count:int",
		RunMake:     false,
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	expected := []string{
		"plugins/gizmos/model.go",
		"plugins/gizmos/presenter.go",
		"plugins/gizmos/dao.go",
		"plugins/gizmos/service.go",
		"plugins/gizmos/mock_dao.go",
		"plugins/gizmos/migration.go",
		"plugins/gizmos/handler.go",
		"plugins/gizmos/plugin.go",
		"plugins/gizmos/integration_test.go",
		"plugins/gizmos/grpc_handler.go",
		"openapi/openapi.gizmos.yaml",
		"proto/rh_trex_ai/v1/gizmos.proto",
	}
	for _, rel := range expected {
		if _, err := os.Stat(filepath.Join(base, rel)); err != nil {
			t.Errorf("expected generated file %s: %v", rel, err)
		}
	}

	model, err := os.ReadFile(filepath.Join(base, "plugins/gizmos/model.go"))
	if err != nil {
		t.Fatalf("read model.go: %v", err)
	}
	for _, want := range []string{"package gizmos", "type Gizmo struct", "Name  *string", "Count *int"} {
		if !strings.Contains(string(model), want) {
			t.Errorf("model.go missing %q", want)
		}
	}

	main, err := os.ReadFile(filepath.Join(cmdDir, "main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !strings.Contains(string(main), `_ "github.com/openshift-online/rh-trex-ai/components/api-server/plugins/gizmos"`) {
		t.Error("main.go missing plugin import")
	}

	openapi, err := os.ReadFile(filepath.Join(openAPIDir, "openapi.yaml"))
	if err != nil {
		t.Fatalf("read openapi.yaml: %v", err)
	}
	if !strings.Contains(string(openapi), "gizmos") {
		t.Error("openapi.yaml missing generated refs")
	}
}
