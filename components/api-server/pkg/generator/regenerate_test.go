package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegenerateIdempotent(t *testing.T) {
	root := seedRepo(t)

	if err := Run(gizmoOptions(root)); err != nil {
		t.Fatalf("seed Run: %v", err)
	}

	m := &Manifest{
		ProtoPackage: "rh_trex",
		Entities:     []EntitySpec{{Kind: "Gizmo", Fields: "name:string,count:int"}},
	}

	openapiPath := filepath.Join(root, "openapi", "openapi.yaml")
	mainPath := filepath.Join(root, "cmd", "trex", "main.go")

	countRefs := func() int {
		data, err := os.ReadFile(openapiPath)
		if err != nil {
			t.Fatalf("read openapi: %v", err)
		}
		return strings.Count(string(data), "openapi.gizmos.yaml")
	}
	countImports := func() int {
		data, err := os.ReadFile(mainPath)
		if err != nil {
			t.Fatalf("read main: %v", err)
		}
		return strings.Count(string(data), "plugins/gizmos")
	}

	if err := Regenerate(gizmoOptions(root), m, root); err != nil {
		t.Fatalf("Regenerate 1: %v", err)
	}
	refs1, imports1 := countRefs(), countImports()
	if refs1 == 0 || imports1 != 1 {
		t.Fatalf("after first regen refs=%d imports=%d, want refs>0 and imports=1", refs1, imports1)
	}

	// Idempotency: second regeneration must not duplicate refs or imports.
	if err := Regenerate(gizmoOptions(root), m, root); err != nil {
		t.Fatalf("Regenerate 2: %v", err)
	}
	if refs2, imports2 := countRefs(), countImports(); refs2 != refs1 || imports2 != imports1 {
		t.Fatalf("regeneration not idempotent: refs %d->%d imports %d->%d", refs1, refs2, imports1, imports2)
	}
}
