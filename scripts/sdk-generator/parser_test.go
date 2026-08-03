package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestRepositoryCharacterization(t *testing.T) {
	spec, err := parseSpec(filepath.Join("..", "..", "openapi", "openapi.yaml"), "/api/rh-trex-ai/v1")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := sdkResourceSummary(spec.Resources), []string{
		"Dinosaur:dinosaurs:species:string:patch",
		"Fossil:fossils:discovery_location:string,estimated_age:int32,excavator_name:string,fossil_type:string:patch",
		"Scientist:scientists:field:string,name:string:patch",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("repository projection = %#v, want %#v", got, want)
	}
}

func TestSharedFixtureConformance(t *testing.T) {
	spec, err := parseSpec(filepath.Join("..", "openapi-ir", "testdata", "conformance", "openapi.yaml"), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Resources) != 1 {
		t.Fatalf("resources = %#v", spec.Resources)
	}
	widget := spec.Resources[0]
	if widget.Name != "Widget" || !widget.HasPatch || !widget.HasDelete || !reflect.DeepEqual(widget.Actions, []string{"archive"}) {
		t.Fatalf("operation-derived resource semantics lost: %#v", widget)
	}
	if !containsSDKField(widget.Fields, "name") || containsSDKField(widget.Fields, "id") || !containsSDKField(widget.PatchFields, "state") {
		t.Fatalf("schema projection is wrong: %#v", widget)
	}
}

func TestGeneratedSDKAcceptanceAndDeterminism(t *testing.T) {
	specPath := filepath.Join("..", "..", "openapi", "openapi.yaml")
	spec, err := parseSpec(specPath, "/api/rh-trex-ai/v1")
	if err != nil {
		t.Fatal(err)
	}
	spec.Module = "github.com/openshift-online/rh-trex-ai-sdk"
	spec.Project = "rh-trex-ai"
	header := GeneratedHeader{SpecPath: specPath, SpecHash: "acceptance", Timestamp: "1970-01-01T00:00:00Z"}

	first := t.TempDir()
	second := t.TempDir()
	generateAllSDKs(t, spec, first, header)
	generateAllSDKs(t, spec, second, header)
	if got, want := hashTree(t, first), hashTree(t, second); !reflect.DeepEqual(got, want) {
		t.Fatalf("generation is not deterministic:\nfirst=%v\nsecond=%v", got, want)
	}

	acceptGeneratedGo(t, filepath.Join(first, "go"))
	acceptGeneratedPython(t, filepath.Join(first, "python"))
	acceptGeneratedTypeScript(t, filepath.Join(first, "typescript"))
}

func generateAllSDKs(t *testing.T, spec *Spec, root string, header GeneratedHeader) {
	t.Helper()
	if err := generateGo(spec, filepath.Join(root, "go"), header); err != nil {
		t.Fatal(err)
	}
	if err := generatePython(spec, filepath.Join(root, "python"), header); err != nil {
		t.Fatal(err)
	}
	if err := generateTypeScript(spec, filepath.Join(root, "typescript"), header); err != nil {
		t.Fatal(err)
	}
}

func acceptGeneratedGo(t *testing.T, root string) {
	t.Helper()
	writeAcceptanceFile(t, filepath.Join(root, "go.mod"), "module github.com/openshift-online/rh-trex-ai-sdk\n\ngo 1.24.0\n")
	acceptance := `package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGeneratedClientRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/api/rh-trex-ai/v1/dinosaurs" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(` + "`" + `{"kind":"DinosaurList","page":1,"size":1,"total":1,"items":[{"species":"T. rex"}]}` + "`" + `))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "token")
	if err != nil { t.Fatal(err) }
	list, err := client.Dinosaurs().List(context.Background(), nil)
	if err != nil { t.Fatal(err) }
	if len(list.Items) != 1 || list.Items[0].Species != "T. rex" { t.Fatalf("list = %#v", list) }
}
`
	writeAcceptanceFile(t, filepath.Join(root, "client", "acceptance_test.go"), acceptance)
	runSDKCommand(t, root, "go", "test", "./...")
}

func acceptGeneratedPython(t *testing.T, root string) {
	t.Helper()
	runSDKCommand(t, root, "python3", "-m", "compileall", "-q", ".")
	runSDKCommand(t, filepath.Dir(root), "python3", "-c", "import python; assert python.Dinosaur(species='T. rex').species == 'T. rex'")
}

func acceptGeneratedTypeScript(t *testing.T, root string) {
	t.Helper()
	writeAcceptanceFile(t, filepath.Join(root, "src", "globals.d.ts"), "declare const process: { env: Record<string, string | undefined> };\n")
	files, err := filepath.Glob(filepath.Join(root, "src", "*.ts"))
	if err != nil {
		t.Fatal(err)
	}
	typescriptVersion := os.Getenv("TREX_TYPESCRIPT_VERSION")
	if typescriptVersion == "" {
		typescriptVersion = "5.3.3"
	}
	arguments := []string{"npm", "exec", "--yes", "--package=typescript@" + typescriptVersion, "--", "tsc", "--noEmit", "--strict", "--target", "ES2022", "--module", "commonjs", "--lib", "ES2022,DOM"}
	for _, file := range files {
		arguments = append(arguments, filepath.Base(file))
	}
	runSDKNodeCommand(t, filepath.Join(root, "src"), arguments...)
}

const defaultSDKNodeImage = "registry.access.redhat.com/ubi9/nodejs-20:1-1778648167@sha256:74cc7b1d13592b1e425074f434b90e470ab209da85fd1fdb8e6e9e4cabaec51a"

func runSDKNodeCommand(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	root, err := filepath.Abs(directory)
	if err != nil {
		t.Fatal(err)
	}
	tool := os.Getenv("TREX_CONTAINER_TOOL")
	if tool == "" {
		tool = "podman"
	}
	image := os.Getenv("TREX_NODE_IMAGE")
	if image == "" {
		image = defaultSDKNodeImage
	}
	containerArguments := []string{"run", "--rm", "--user", "0", "-v", root + ":/work:Z", "-w", "/work", image}
	containerArguments = append(containerArguments, arguments...)
	return runSDKCommand(t, directory, tool, containerArguments...)
}

func runSDKCommand(t *testing.T, directory, name string, arguments ...string) string {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = directory
	command.Env = append(os.Environ(), "GOWORK=off")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, arguments, err, output)
	}
	return string(output)
}

func writeAcceptanceFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func hashTree(t *testing.T, root string) []string {
	t.Helper()
	var result []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result = append(result, fmt.Sprintf("%x  %s", sha256.Sum256(data), filepath.ToSlash(relative)))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(result)
	return result
}

func sdkResourceSummary(resources []Resource) []string {
	result := make([]string, 0, len(resources))
	for _, resource := range resources {
		fields := make([]string, 0, len(resource.Fields))
		for _, field := range resource.Fields {
			fields = append(fields, field.Name+":"+field.GoType)
		}
		capability := ""
		if resource.HasPatch {
			capability = "patch"
		}
		result = append(result, resource.Name+":"+resource.PathSegment+":"+strings.Join(fields, ",")+":"+capability)
	}
	return result
}

func containsSDKField(fields []Field, name string) bool {
	for _, field := range fields {
		if field.Name == name {
			return true
		}
	}
	return false
}
