package ir

import (
	"reflect"
	"strings"
	"testing"
)

func TestOperationAndSecurityConformance(t *testing.T) {
	document := loadConformance(t)

	public := requiredOperation(t, document, "listWidgets")
	if public.Security.State != SecurityNone {
		t.Fatalf("list security = %q, want none", public.Security.State)
	}
	if public.Method != "GET" || public.Path != "/widgets" || public.Summary == "" || len(public.Tags) != 1 {
		t.Fatalf("operation metadata lost: %#v", public)
	}
	if len(public.Parameters) != 1 || public.Parameters[0].Style != "deepObject" || !public.Parameters[0].Explode {
		t.Fatalf("query serialization lost: %#v", public.Parameters)
	}

	inherited := requiredOperation(t, document, "createWidget")
	if inherited.Security.State != SecurityInherited {
		t.Fatalf("create security = %q, want inherited", inherited.Security.State)
	}
	override := requiredOperation(t, document, "patchWidget")
	if override.Security.State != SecurityOverride || len(override.Security.Requirements) != 1 || override.Security.Requirements[0].Schemes[0].Scopes[0] != "widgets:write" {
		t.Fatalf("override security lost: %#v", override.Security)
	}
	if !override.Deprecated || len(override.RequestBody.Content) != 1 || override.RequestBody.Content[0].ContentType != "application/merge-patch+json" {
		t.Fatalf("operation fidelity lost: %#v", override)
	}

	nested := requiredOperation(t, document, "listProjectWidgets")
	if got, want := parameterNames(nested.PathParameters), []string{"organization_id", "project_id"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ordered nested path parameters = %v, want %v", got, want)
	}
	action := requiredOperation(t, document, "archiveWidget")
	if !action.Capabilities.Has(CapabilityAction) {
		t.Fatalf("action capability missing: %v", action.Capabilities)
	}
	stream := requiredOperation(t, document, "streamWidgetEvents")
	if !stream.Capabilities.Has(CapabilityStream) {
		t.Fatalf("stream capability missing: %v", stream.Capabilities)
	}
}

func TestSchemaAndUsageRoleConformance(t *testing.T) {
	document := loadConformance(t)
	widget := namedSchema(t, document, "Widget")
	if len(widget.AllOf) != 2 || widget.Extensions["x-schema-extension"].Value != "widget-schema" {
		t.Fatalf("composition or schema extension lost: %#v", widget)
	}
	properties := document.EffectiveProperties(widget.Ref)
	if !properties["id"].ReadOnly || !properties["secret"].WriteOnly || !properties["state"].Nullable {
		t.Fatalf("read/write/null semantics lost: %#v", properties)
	}
	if properties["child"].Schema.Ref != widget.Ref {
		t.Fatalf("recursive identity lost: %#v", properties["child"])
	}
	nameSchema := document.Schema(properties["name"].Schema.Ref)
	if nameSchema.MinLength != 1 || nameSchema.MaxLength == nil || nameSchema.Pattern == "" || nameSchema.Extensions["x-property-extension"].Value != "widget-name" {
		t.Fatalf("string constraints or property extension lost: %#v", nameSchema)
	}
	stateSchema := document.Schema(properties["state"].Schema.Ref)
	if len(stateSchema.Enum) != 3 || stateSchema.Default != "new" || stateSchema.Example != "active" {
		t.Fatalf("enum/default/example lost: %#v", stateSchema)
	}
	labelsSchema := document.Schema(properties["labels"].Schema.Ref)
	if labelsSchema.AdditionalProperties == nil {
		t.Fatalf("map schema lost: %#v", labelsSchema)
	}
	event := namedSchema(t, document, "Event")
	if len(event.OneOf) != 2 {
		t.Fatalf("oneOf lost: %#v", event)
	}

	roles := rolesFor(document, widget.Ref)
	for _, expected := range []SchemaRole{SchemaRoleResponse, SchemaRoleListItem} {
		if !roles[expected] {
			t.Fatalf("Widget lacks role %q: %#v", expected, roles)
		}
	}
	errorSchema := namedSchema(t, document, "Error")
	if !rolesFor(document, errorSchema.Ref)[SchemaRoleError] {
		t.Fatalf("Error lacks error role: %#v", rolesFor(document, errorSchema.Ref))
	}
	createSchema := namedSchema(t, document, "WidgetCreate")
	if !rolesFor(document, createSchema.Ref)[SchemaRoleRequest] {
		t.Fatalf("WidgetCreate lacks request role")
	}
	for _, view := range document.ResourceViews {
		if view.SchemaRef == errorSchema.Ref || view.SchemaRef == createSchema.Ref {
			t.Fatalf("helper schema became a resource view: %#v", view)
		}
	}
}

func TestResourceGraphAndMetadataConformance(t *testing.T) {
	document := loadConformance(t)
	widget := namedSchema(t, document, "Widget")
	collections := make(map[string]*ResourceView)
	for _, view := range document.ResourceViews {
		if view.SchemaRef == widget.Ref && view.Kind == ResourceCollection {
			collections[view.Path] = view
		}
	}
	for _, path := range []string{"/widgets", "/organizations/{organization_id}/projects/{project_id}/widgets", "/accounts/{account_id}/widgets"} {
		if collections[path] == nil {
			t.Fatalf("scoped collection %q missing: %#v", path, collections)
		}
	}
	if got := collections["/organizations/{organization_id}/projects/{project_id}/widgets"].ScopeParameters; !reflect.DeepEqual(got, []string{"organization_id", "project_id"}) {
		t.Fatalf("scope parameters = %v", got)
	}

	var explicit, inferred bool
	for _, relationship := range document.Relationships {
		if relationship.Provenance == RelationshipExplicit && relationship.SourceOperationID == "listWidgets" && relationship.TargetOperationID == "getWidget" {
			explicit = len(relationship.ParameterMappings) == 1
		}
		if relationship.Provenance == RelationshipInferred && relationship.TargetOperationID == "listAccountWidgets" {
			inferred = true
		}
		if relationship.Provenance == RelationshipInferred && relationship.TargetOperationID == "listProjectWidgets" {
			t.Fatalf("ambiguous organization/project containment was inferred: %#v", relationship)
		}
	}
	if !explicit || !inferred {
		t.Fatalf("relationship provenance incomplete: %#v", document.Relationships)
	}

	list := requiredOperation(t, document, "listWidgets")
	if document.Extensions["x-document-extension"].Value != "document-value" || list.PathExtensions["x-path-extension"].Value != "widgets-path" || list.Extensions["x-operation-extension"].Value != "list-widgets" || list.Parameters[0].Extensions["x-parameter-extension"].Value != "filter-parameter" {
		t.Fatalf("extensions were not preserved at all scopes: document=%v path=%v operation=%v parameter=%v", document.Extensions, list.PathExtensions, list.Extensions, list.Parameters[0].Extensions)
	}
}

func TestSafeProjection(t *testing.T) {
	document := loadConformance(t)
	if err := document.ValidateProjectionNames(); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	path, err := SafeJoin(root, "nested", "output.go")
	if err != nil || !strings.HasPrefix(path, root) {
		t.Fatalf("safe path = %q, %v", path, err)
	}
	if _, err := SafeJoin(root, "..", "escaped.go"); err == nil {
		t.Fatal("path traversal was accepted")
	}
	for _, unsafe := range []string{"bad/name", `bad\"; os.Exit(1); //`, "{{template .}}", "$(touch owned)"} {
		if err := ValidateIdentifier(unsafe); err == nil {
			t.Fatalf("unsafe identifier %q accepted", unsafe)
		}
	}
}

func requiredOperation(t *testing.T, document *Document, id string) *Operation {
	t.Helper()
	operation := document.Operation(id)
	if operation == nil {
		t.Fatalf("operation %s not found", id)
	}
	return operation
}

func parameterNames(parameters []*Parameter) []string {
	result := make([]string, 0, len(parameters))
	for _, parameter := range parameters {
		result = append(result, parameter.Name)
	}
	return result
}

func rolesFor(document *Document, ref string) map[SchemaRole]bool {
	result := make(map[SchemaRole]bool)
	for _, use := range document.SchemaUses {
		if use.SchemaRef == ref {
			result[use.Role] = true
		}
	}
	return result
}
