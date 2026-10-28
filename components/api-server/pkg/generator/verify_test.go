package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openshift-online/rh-trex-ai/components/api-server/templates"
)

const verifySeedMain = `package main

import (
	"github.com/spf13/cobra"
)

func main() {}
`

const verifySeedOpenAPI = "paths:\n  # AUTO-ADD NEW PATHS\ncomponents:\n  schemas:\n    # AUTO-ADD NEW SCHEMAS\n"

func seedRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	cmdDir := filepath.Join(root, "cmd", "trex")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("mkdir cmd: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(verifySeedMain), 0644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	openapiDir := filepath.Join(root, "openapi")
	if err := os.MkdirAll(openapiDir, 0755); err != nil {
		t.Fatalf("mkdir openapi: %v", err)
	}
	if err := os.WriteFile(filepath.Join(openapiDir, "openapi.yaml"), []byte(verifySeedOpenAPI), 0644); err != nil {
		t.Fatalf("write openapi.yaml: %v", err)
	}

	return root
}

func verifyOptions(root string) Options {
	return Options{
		TemplatesFS:  templates.GeneratorFS,
		BaseDir:      root,
		Repo:         "github.com/openshift-online",
		Project:      "rh-trex-ai",
		Library:      "github.com/openshift-online/rh-trex-ai/components/api-server",
		ProtoPackage: "rh_trex",
		Quiet:        true,
	}
}

func gizmoOptions(root string) Options {
	opts := verifyOptions(root)
	opts.Kind = "Gizmo"
	opts.Fields = "name:string,count:int"
	return opts
}

func TestVerifyNoDrift(t *testing.T) {
	root := seedRepo(t)

	if err := Run(gizmoOptions(root)); err != nil {
		t.Fatalf("seed Run: %v", err)
	}

	m := &Manifest{ProtoPackage: "rh_trex", Entities: []EntitySpec{{Kind: "Gizmo", Fields: "name:string,count:int"}}}
	drift, err := Verify(gizmoOptions(root), m, root)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(drift) != 0 {
		t.Errorf("expected no drift, got:\n%s", strings.Join(drift, "\n"))
	}
}

func TestVerifyDetectsDriftAndIgnore(t *testing.T) {
	root := seedRepo(t)

	if err := Run(gizmoOptions(root)); err != nil {
		t.Fatalf("seed Run: %v", err)
	}

	modelPath := filepath.Join(root, "plugins", "gizmos", "model.go")
	data, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("read model.go: %v", err)
	}
	if err := os.WriteFile(modelPath, append(data, []byte("// manual edit\n")...), 0644); err != nil {
		t.Fatalf("modify model.go: %v", err)
	}

	m := &Manifest{ProtoPackage: "rh_trex", Entities: []EntitySpec{{Kind: "Gizmo", Fields: "name:string,count:int"}}}
	drift, err := Verify(gizmoOptions(root), m, root)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(drift) != 1 || !strings.Contains(drift[0], "plugins/gizmos/model.go") {
		t.Errorf("expected model.go drift, got: %v", drift)
	}

	m.Ignore = []string{"plugins/gizmos/model.go"}
	drift, err = Verify(gizmoOptions(root), m, root)
	if err != nil {
		t.Fatalf("Verify with ignore: %v", err)
	}
	if len(drift) != 0 {
		t.Errorf("expected no drift when ignored, got: %v", drift)
	}
}

func TestVerifyMissingFile(t *testing.T) {
	root := seedRepo(t)

	if err := Run(gizmoOptions(root)); err != nil {
		t.Fatalf("seed Run: %v", err)
	}

	if err := os.Remove(filepath.Join(root, "plugins", "gizmos", "service.go")); err != nil {
		t.Fatalf("remove service.go: %v", err)
	}

	m := &Manifest{ProtoPackage: "rh_trex", Entities: []EntitySpec{{Kind: "Gizmo", Fields: "name:string,count:int"}}}
	drift, err := Verify(gizmoOptions(root), m, root)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	found := false
	for _, d := range drift {
		if strings.Contains(d, "plugins/gizmos/service.go") && strings.Contains(d, "missing") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected service.go missing drift, got: %v", drift)
	}
}
