package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func writeManifest(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "entities.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func TestLoadManifest(t *testing.T) {
	path := writeManifest(t, `proto_package: rh_trex
ignore:
  - "plugins/*/grpc_integration_test.go"
entities:
  - kind: Dinosaur
    fields: "species:string:required"
  - kind: Fossil
    fields: "discovery_location:string:required,estimated_age:int"
`)

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.ProtoPackage != "rh_trex" {
		t.Errorf("ProtoPackage = %q, want rh_trex", m.ProtoPackage)
	}
	if len(m.Entities) != 2 {
		t.Fatalf("len(Entities) = %d, want 2", len(m.Entities))
	}
	if m.Entities[0].Kind != "Dinosaur" || m.Entities[0].Fields != "species:string:required" {
		t.Errorf("unexpected first entity: %+v", m.Entities[0])
	}
	if !m.ignores("plugins/dinosaurs/grpc_integration_test.go") {
		t.Error("ignore glob did not match plugins/dinosaurs/grpc_integration_test.go")
	}
	if m.ignores("plugins/dinosaurs/service.go") {
		t.Error("ignore glob unexpectedly matched service.go")
	}
}

func TestLoadManifestErrors(t *testing.T) {
	if _, err := LoadManifest(writeManifest(t, "entities: []")); err == nil {
		t.Error("expected error for empty entities")
	}
	if _, err := LoadManifest(writeManifest(t, "entities:\n  - fields: \"x:string\"")); err == nil {
		t.Error("expected error for entity without kind")
	}
	if _, err := LoadManifest(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Error("expected error for missing file")
	}
}
