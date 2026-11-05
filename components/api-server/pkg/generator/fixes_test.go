package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateKind(t *testing.T) {
	valid := []struct{ kind, plural string }{
		{"Dinosaur", ""},
		{"Goose", "Geese"},
		{"APIKey", ""},
	}
	for _, tt := range valid {
		if err := ValidateKind(tt.kind, tt.plural); err != nil {
			t.Errorf("ValidateKind(%q, %q) = %v, want nil", tt.kind, tt.plural, err)
		}
	}

	invalid := []struct{ kind, plural string }{
		{"", ""},
		{"dinosaur", ""},
		{"../../escape", ""},
		{"Dino-saur", ""},
		{"Dino saur", ""},
		{"Goose", "../geese"},
	}
	for _, tt := range invalid {
		if err := ValidateKind(tt.kind, tt.plural); err == nil {
			t.Errorf("ValidateKind(%q, %q) = nil, want error", tt.kind, tt.plural)
		}
	}
}

func TestRunRejectsInvalidKindWithoutWriting(t *testing.T) {
	root := seedRepo(t)
	opts := verifyOptions(root)
	opts.Kind = "../../escape"
	if err := Run(opts); err == nil {
		t.Fatal("Run with a path-like kind succeeded, want error")
	}
	if _, err := os.Stat(filepath.Join(root, "plugins")); !os.IsNotExist(err) {
		t.Errorf("Run with an invalid kind wrote plugins/ (stat err: %v)", err)
	}

	opts.Kind = ""
	if err := Run(opts); err == nil {
		t.Fatal("Run with an empty kind succeeded, want error")
	}
}

func TestRunTwiceDoesNotDuplicateOpenAPIRefs(t *testing.T) {
	root := seedRepo(t)
	for i := 0; i < 2; i++ {
		if err := Run(gizmoOptions(root)); err != nil {
			t.Fatalf("Run #%d: %v", i+1, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "openapi", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "$ref") && strings.Count(string(data), line+"\n") > 1 {
			t.Fatalf("openapi.yaml has a duplicated line after two runs: %q\n%s", line, data)
		}
	}
}

func TestRunFailsWhenOpenAPIMarkerMissing(t *testing.T) {
	root := seedRepo(t)
	if err := os.WriteFile(filepath.Join(root, "openapi", "openapi.yaml"), []byte("paths: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	err := Run(gizmoOptions(root))
	if err == nil || !strings.Contains(err.Error(), "AUTO-ADD NEW PATHS") {
		t.Fatalf("Run without the marker = %v, want a missing-marker error", err)
	}
}

func TestExplicitPluralNamesPackageAndDirectory(t *testing.T) {
	root := seedRepo(t)
	opts := verifyOptions(root)
	opts.Kind = "Goose"
	opts.Plural = "Geese"
	opts.Fields = "name:string"
	if err := Run(opts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	model, err := os.ReadFile(filepath.Join(root, "plugins", "geese", "model.go"))
	if err != nil {
		t.Fatalf("expected plugins/geese/model.go: %v", err)
	}
	if !strings.Contains(string(model), "package geese") {
		t.Errorf("model.go has the wrong package:\n%s", model)
	}
	if _, err := os.Stat(filepath.Join(root, "openapi", "openapi.geese.yaml")); err != nil {
		t.Errorf("expected openapi/openapi.geese.yaml: %v", err)
	}
	if n := ComputeNaming("Goose", "Geese"); n.KindLowerPlural != "geese" {
		t.Errorf("ComputeNaming KindLowerPlural = %q, want geese", n.KindLowerPlural)
	}
}

func TestEntityOptionsProtoPackagePrecedence(t *testing.T) {
	base := Options{Project: "my-svc"}
	tests := []struct {
		name   string
		flag   string
		m      string
		entity string
		want   string
	}{
		{"entity wins", "flag_pkg", "manifest_pkg", "entity_pkg", "entity_pkg"},
		{"flag beats manifest", "flag_pkg", "manifest_pkg", "", "flag_pkg"},
		{"manifest when no flag", "", "manifest_pkg", "", "manifest_pkg"},
		{"derived from project", "", "", "", "my_svc"},
	}
	for _, tt := range tests {
		opts := base
		opts.ProtoPackage = tt.flag
		got := EntityOptions(opts, &Manifest{ProtoPackage: tt.m}, EntitySpec{Kind: "Gizmo", ProtoPackage: tt.entity})
		if got.ProtoPackage != tt.want {
			t.Errorf("%s: ProtoPackage = %q, want %q", tt.name, got.ProtoPackage, tt.want)
		}
	}
}
