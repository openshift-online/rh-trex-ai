package ir

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoaderConformance(t *testing.T) {
	t.Run("single document", func(t *testing.T) {
		document, err := Load("testdata/single.yaml", LoadOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(document.Operations) != 1 || document.Operation("listThings") == nil {
			t.Fatalf("unexpected operations: %#v", document.Operations)
		}
	})

	t.Run("split document and recursive schema", func(t *testing.T) {
		document := loadConformance(t)
		widget := namedSchema(t, document, "Widget")
		properties := document.EffectiveProperties(widget.Ref)
		if properties["child"] == nil || properties["child"].Schema.Ref != widget.Ref {
			t.Fatalf("recursive Widget identity was not retained: %#v", properties["child"])
		}
	})

	for _, testCase := range []struct {
		name, path, contains string
	}{
		{"unresolved reference", "testdata/invalid/unresolved.yaml", "/components/schemas/Missing"},
		{"cyclic non-schema reference", "testdata/invalid/cyclic-path.yaml", "cyclic non-schema"},
		{"missing operation id", "testdata/invalid/missing-operation-id.yaml", "/paths/~1things/get"},
		{"duplicate operation id", "testdata/invalid/duplicate-operation-id.yaml", "first declared"},
		{"missing path parameter", "testdata/invalid/missing-path-parameter.yaml", "thing_id"},
		{"unresolved operation link", "testdata/invalid/unresolved-operation-link.yaml", "getMissingThing"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := Load(testCase.path, LoadOptions{})
			if err == nil || !strings.Contains(err.Error(), testCase.contains) {
				t.Fatalf("error = %v, want text %q", err, testCase.contains)
			}
		})
	}
}

func TestBoundedReferenceResolution(t *testing.T) {
	t.Run("parent traversal", func(t *testing.T) {
		root, outside := boundaryFixture(t, false)
		_, err := Load(root, LoadOptions{})
		assertBoundaryDiagnostic(t, err, outside)
	})

	t.Run("symbolic link escape", func(t *testing.T) {
		root, outside := boundaryFixture(t, true)
		_, err := Load(root, LoadOptions{})
		assertBoundaryDiagnostic(t, err, outside)
	})

	t.Run("absolute reference", func(t *testing.T) {
		directory := t.TempDir()
		target := filepath.Join(directory, "target.yaml")
		writeFile(t, target, "type: object\n")
		root := filepath.Join(directory, "openapi.yaml")
		writeFile(t, root, validEmptyDocument("    Escape: {$ref: \""+target+"\"}\n"))
		_, err := Load(root, LoadOptions{})
		if err == nil || !strings.Contains(err.Error(), "absolute file reference") {
			t.Fatalf("error = %v, want absolute-reference rejection", err)
		}
	})

	t.Run("non-file URI", func(t *testing.T) {
		directory := t.TempDir()
		root := filepath.Join(directory, "openapi.yaml")
		writeFile(t, root, validEmptyDocument("    Escape: {$ref: \"https://example.test/schema.yaml\"}\n"))
		_, err := Load(root, LoadOptions{})
		if err == nil || !strings.Contains(err.Error(), "non-file URI") {
			t.Fatalf("error = %v, want URI rejection", err)
		}
	})
}

func TestDeterministicNormalization(t *testing.T) {
	first := loadConformance(t)
	second := loadConformance(t)
	firstJSON, err := json.MarshalIndent(first, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := json.MarshalIndent(second, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatal("unchanged input produced different canonical JSON")
	}
}

func TestUnresolvedOperationLinkDiagnostic(t *testing.T) {
	_, err := Load("testdata/invalid/unresolved-operation-link.yaml", LoadOptions{})
	if err == nil {
		t.Fatal("unresolved operation link unexpectedly normalized")
	}
	for _, expected := range []string{
		"#/paths/~1things/get/responses/200/links/missingThing",
		"link missingThing",
		`target operationId "getMissingThing" was not found`,
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("diagnostic = %v, want text %q", err, expected)
		}
	}
}

func TestRepositoryOpenAPISmoke(t *testing.T) {
	document, err := Load(filepath.Join("..", "..", "openapi", "openapi.yaml"), LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(document.Operations), 15; got != want {
		t.Fatalf("repository operations = %d, want %d", got, want)
	}
	operationIDs := make(map[string]struct{}, len(document.Operations))
	for _, operation := range document.Operations {
		operationIDs[operation.ID] = struct{}{}
	}
	for _, operationID := range []string{
		"listDinosaurs", "createDinosaur", "getDinosaur", "updateDinosaur", "deleteDinosaur",
		"listFossils", "createFossil", "getFossil", "updateFossil", "deleteFossil",
		"listScientists", "createScientist", "getScientist", "updateScientist", "deleteScientist",
	} {
		if _, exists := operationIDs[operationID]; !exists {
			t.Errorf("repository operation %s not normalized", operationID)
		}
	}
	for _, name := range []string{"Dinosaur", "Fossil", "Scientist"} {
		if namedSchemaOrNil(document, name) == nil {
			t.Fatalf("repository schema %s not normalized", name)
		}
	}
}

func boundaryFixture(t *testing.T, symbolicLink bool) (string, string) {
	t.Helper()
	parent := t.TempDir()
	directory := filepath.Join(parent, "document")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "outside.yaml")
	writeFile(t, outside, "secret-marker-that-must-not-leak\n")
	reference := "../outside.yaml"
	if symbolicLink {
		link := filepath.Join(directory, "linked.yaml")
		if err := os.Symlink(outside, link); err != nil {
			t.Fatal(err)
		}
		reference = "linked.yaml"
	}
	root := filepath.Join(directory, "openapi.yaml")
	writeFile(t, root, validEmptyDocument("    Escape: {$ref: \""+reference+"\"}\n"))
	return root, outside
}

func assertBoundaryDiagnostic(t *testing.T, err error, outside string) {
	t.Helper()
	if err == nil {
		t.Fatal("reference escape unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "/components/schemas/Escape") || !strings.Contains(err.Error(), "outside configured document roots") {
		t.Fatalf("non-actionable boundary diagnostic: %v", err)
	}
	contents, readErr := os.ReadFile(outside)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.Contains(err.Error(), strings.TrimSpace(string(contents))) {
		t.Fatalf("diagnostic leaked target contents: %v", err)
	}
}

func validEmptyDocument(schemas string) string {
	return "openapi: 3.0.3\ninfo: {title: boundary, version: 1.0.0}\npaths: {}\ncomponents:\n  schemas:\n" + schemas
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func loadConformance(t *testing.T) *Document {
	t.Helper()
	document, err := Load("testdata/conformance/openapi.yaml", LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return document
}

func namedSchema(t *testing.T, document *Document, name string) *Schema {
	t.Helper()
	schema := namedSchemaOrNil(document, name)
	if schema == nil {
		t.Fatalf("schema %s not found", name)
	}
	return schema
}

func namedSchemaOrNil(document *Document, name string) *Schema {
	for _, schema := range document.Schemas {
		if schema.Name == name {
			return schema
		}
	}
	return nil
}
