package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRepositoryCharacterization(t *testing.T) {
	resources, err := parseResources(filepath.Join("..", "..", "openapi", "openapi.yaml"), "/api/rh-trex-ai/v1")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := resourceSummary(resources), []string{
		"Dinosaur:dinosaurs:species:string",
		"Fossil:fossils:discovery_location:string,estimated_age:int,excavator_name:string,fossil_type:string",
		"Scientist:scientists:field:string,name:string",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("repository projection = %#v, want %#v", got, want)
	}
	if resources[0].DefaultColumns != "id, species, created_at" || resources[1].DefaultColumns != "id, discovery_location, estimated_age, excavator_name, fossil_type, created_at" {
		t.Fatalf("legacy columns changed: %#v", resources)
	}
}

func TestSharedFixtureConformance(t *testing.T) {
	resources, err := parseResources(filepath.Join("..", "openapi-ir", "testdata", "conformance", "openapi.yaml"), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 1 || resources[0].Name != "Widget" || resources[0].PathSegment != "widgets" {
		t.Fatalf("shared fixture projection = %#v", resources)
	}
	if !containsCLIField(resources[0].WritableFields, "name") || containsCLIField(resources[0].WritableFields, "id") {
		t.Fatalf("schema-derived writable fields are wrong: %#v", resources[0].WritableFields)
	}
}

func TestGeneratedCLIAcceptance(t *testing.T) {
	resources, err := parseResources(filepath.Join("..", "..", "openapi", "openapi.yaml"), "/api/rh-trex-ai/v1")
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	data := cliData{
		Binary: "trex-cli", Project: "rh-trex-ai", APIPrefix: "/api/rh-trex-ai/v1",
		Module: "github.com/openshift-online/rh-trex-ai-cli", Resources: resources,
	}
	if err := generateCLI(data, output); err != nil {
		t.Fatal(err)
	}
	runCommand(t, output, "go", "mod", "tidy")
	runCommand(t, output, "go", "test", "./...")
	binary := filepath.Join(output, "trex-cli")
	runCommand(t, output, "go", "build", "-o", binary, "./cmd/trex-cli")
	help := runCommand(t, output, binary, "list", "dinosaurs", "--help")
	if !strings.Contains(help, "List dinosaurs") || !strings.Contains(help, "--columns") {
		t.Fatalf("generated command behavior changed:\n%s", help)
	}
	generated := readTestFile(t, filepath.Join(output, "pkg", "urls", "urls.go"))
	if !strings.Contains(generated, `DinosaursPath = APIPrefix + "/dinosaurs"`) {
		t.Fatalf("generated list route is not exact:\n%s", generated)
	}
}

func resourceSummary(resources []cliResource) []string {
	result := make([]string, 0, len(resources))
	for _, resource := range resources {
		fields := make([]string, 0, len(resource.WritableFields))
		for _, field := range resource.WritableFields {
			fields = append(fields, field.Name+":"+field.GoType)
		}
		result = append(result, resource.Name+":"+resource.PathSegment+":"+strings.Join(fields, ","))
	}
	return result
}

func containsCLIField(fields []cliField, name string) bool {
	for _, field := range fields {
		if field.Name == name {
			return true
		}
	}
	return false
}
