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
