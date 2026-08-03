package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestRepositoryCharacterization(t *testing.T) {
	resources, err := parseResources(filepath.Join("..", "..", "openapi", "openapi.yaml"), "/api/rh-trex-ai/v1")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := pluginResourceSummary(resources), []string{
		"Dinosaur:dinosaurs:3:species:string:patch",
		"Fossil:fossils:6:discovery_location:string,estimated_age:integer,excavator_name:string,fossil_type:string:patch",
		"Scientist:scientists:4:field:string,name:string:patch",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("repository projection = %#v, want %#v", got, want)
	}
}

func TestSharedFixtureConformance(t *testing.T) {
	resources, err := parseResources(filepath.Join("..", "openapi-ir", "testdata", "conformance", "openapi.yaml"), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 1 {
		t.Fatalf("resources = %#v", resources)
	}
	widget := resources[0]
	if widget.Name != "Widget" || widget.PathSegment != "widgets" || !widget.HasPatch || !widget.HasDelete {
		t.Fatalf("resource operation semantics lost: %#v", widget)
	}
	if !containsPluginField(widget.WritableFields, "name") || containsPluginField(widget.WritableFields, "id") {
		t.Fatalf("writable field projection is wrong: %#v", widget.WritableFields)
	}
}

func TestGeneratedConsoleAcceptanceAndDeterminism(t *testing.T) {
	resources, err := parseResources(filepath.Join("..", "..", "openapi", "openapi.yaml"), "/api/rh-trex-ai/v1")
	if err != nil {
		t.Fatal(err)
	}
	data := pluginData{
		PluginName: "rh-trex-ai-console", DisplayName: "Rh Trex Ai Console", Project: "rh-trex-ai",
		APIPrefix: "/api/rh-trex-ai/v1", NavSection: "home", Perspective: "admin", Resources: resources,
	}
	first, second := t.TempDir(), t.TempDir()
	if err := generatePlugin(data, first); err != nil {
		t.Fatal(err)
	}
	if err := generatePlugin(data, second); err != nil {
		t.Fatal(err)
	}
	if got, want := pluginHashTree(t, first), pluginHashTree(t, second); !reflect.DeepEqual(got, want) {
		t.Fatalf("console generation is not deterministic:\nfirst=%v\nsecond=%v", got, want)
	}

	var extensions []map[string]any
	extensionData, err := os.ReadFile(filepath.Join(first, "console-extensions.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(extensionData, &extensions); err != nil {
		t.Fatalf("generated console extensions are invalid JSON: %v", err)
	}
	if len(extensions) == 0 || !strings.Contains(string(extensionData), "/dinosaurs") {
		t.Fatalf("generated navigation/pages missing: %s", extensionData)
	}
	assertPinnedPluginDependencies(t, first)
	details := readPluginFile(t, filepath.Join(first, "src", "components", "DinosaurDetailsPage.tsx"))
	if !strings.Contains(details, "createAPIClient") || !strings.Contains(details, "api.dinosaurs.get(id)") {
		t.Fatalf("generated component lacks API behavior:\n%s", details)
	}

	runPluginNodeCommand(t, first, "npm", "ci", "--ignore-scripts", "--no-audit", "--no-fund")
	runPluginNodeCommand(t, first, "npm", "run", "build")
}

func assertPinnedPluginDependencies(t *testing.T, root string) {
	t.Helper()
	var manifest struct {
		PackageManager  string            `json:"packageManager"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	manifestData, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("generated package.json is invalid: %v", err)
	}
	exactVersion := regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	if manifest.PackageManager != "npm@10.8.2" {
		t.Fatalf("package manager is not pinned: %q", manifest.PackageManager)
	}
	for name, version := range manifest.DevDependencies {
		if !exactVersion.MatchString(version) {
			t.Fatalf("dependency %s is not exact: %q", name, version)
		}
	}

	var lock struct {
		LockfileVersion int `json:"lockfileVersion"`
		Packages        map[string]struct {
			DevDependencies map[string]string `json:"devDependencies"`
		} `json:"packages"`
	}
	lockData, err := os.ReadFile(filepath.Join(root, "package-lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(lockData, &lock); err != nil {
		t.Fatalf("generated package-lock.json is invalid: %v", err)
	}
	if lock.LockfileVersion != 3 || !reflect.DeepEqual(lock.Packages[""].DevDependencies, manifest.DevDependencies) {
		t.Fatal("generated lockfile does not match exact root dependency declarations")
	}
	dockerfile := readPluginFile(t, filepath.Join(root, "Dockerfile"))
	if strings.Contains(dockerfile, ":latest") || strings.Count(dockerfile, "@sha256:") != 2 {
		t.Fatalf("generated container images are not immutable:\n%s", dockerfile)
	}
}

const defaultNodeImage = "registry.access.redhat.com/ubi9/nodejs-20:1-1778648167@sha256:74cc7b1d13592b1e425074f434b90e470ab209da85fd1fdb8e6e9e4cabaec51a"

func runPluginNodeCommand(t *testing.T, directory string, arguments ...string) string {
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
		image = defaultNodeImage
	}
	containerArguments := []string{"run", "--rm", "--user", "0", "-v", root + ":/work:Z", "-w", "/work", image}
	containerArguments = append(containerArguments, arguments...)
	return runPluginCommand(t, directory, tool, containerArguments...)
}

func runPluginCommand(t *testing.T, directory, name string, arguments ...string) string {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, arguments, err, output)
	}
	return string(output)
}

func readPluginFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func pluginHashTree(t *testing.T, root string) []string {
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

func pluginResourceSummary(resources []pluginResource) []string {
	result := make([]string, 0, len(resources))
	for _, resource := range resources {
		fields := make([]string, 0, len(resource.WritableFields))
		for _, field := range resource.WritableFields {
			fields = append(fields, field.JSONName+":"+field.FieldType)
		}
		capability := ""
		if resource.HasPatch {
			capability = "patch"
		}
		result = append(result, fmt.Sprintf("%s:%s:%d:%s:%s", resource.Name, resource.PathSegment, len(resource.Columns), strings.Join(fields, ","), capability))
	}
	return result
}

func containsPluginField(fields []pluginField, name string) bool {
	for _, field := range fields {
		if field.JSONName == name {
			return true
		}
	}
	return false
}
