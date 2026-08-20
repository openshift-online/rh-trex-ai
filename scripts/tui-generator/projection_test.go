package main

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/openshift-online/rh-trex-ai/pkg/tui"
	ir "github.com/openshift-online/rh-trex-ai/scripts/openapi-ir"
)

func TestNavigationProjectionGraphConformance(t *testing.T) {
	descriptor := loadProjectedFixture(t, "testdata/navigation.yaml")

	parents := viewWithOperation(t, descriptor, "listParents")
	if parents.Label != "Parents" || !reflect.DeepEqual(parents.Aliases, []string{"pa"}) || parents.IdentityProperty != "id" || parents.DefaultSort != "name" {
		t.Fatalf("typed presentation was not retained: %#v", parents)
	}
	if got := columnSummary(parents.Columns); !reflect.DeepEqual(got, []string{"name:NAME:100", "id:ID:90"}) {
		t.Fatalf("columns = %#v", got)
	}
	if parents.Columns[0].Type != "string" || parents.Columns[1].Type != "string" {
		t.Fatalf("column schema types were not retained: %#v", parents.Columns)
	}

	scopedChildren := viewWithOperation(t, descriptor, "listChildren")
	parentItem := viewWithOperation(t, descriptor, "getParent")
	accountItem := viewWithOperation(t, descriptor, "getAccount")
	globalChildren := viewWithOperation(t, descriptor, "listGlobalChildren")
	if len(globalChildren.ScopeParameters) != 0 {
		t.Fatalf("global children unexpectedly scoped: %#v", globalChildren.ScopeParameters)
	}
	if !operationByID(t, descriptor, "listGlobalChildren").Security.None || operationByID(t, descriptor, "listParents").Security.None {
		t.Fatal("explicit public security and inherited bearer security were conflated")
	}

	assertExplicitRuntimeEdge(t, descriptor, parentItem.ID, scopedChildren.ID, "getParent", "$response.body#/id")
	assertExplicitRuntimeEdge(t, descriptor, accountItem.ID, scopedChildren.ID, "getAccount", "$response.body#/id")
	explicitParentEdges := 0
	for _, edge := range descriptor.Edges {
		if edge.SourceViewID == parentItem.ID && edge.TargetViewID == scopedChildren.ID {
			explicitParentEdges++
			if edge.Provenance != "explicit-link" {
				t.Fatalf("inferred edge survived explicit precedence: %#v", edge)
			}
		}
	}
	if explicitParentEdges != 1 {
		t.Fatalf("parent-to-children edge count = %d, want one explicit edge", explicitParentEdges)
	}

	childItem := viewWithOperation(t, descriptor, "getChild")
	itemEdge := edgeBetween(t, descriptor, scopedChildren.ID, childItem.ID)
	if itemEdge.Provenance != "collection-item" || !itemEdge.Navigable {
		t.Fatalf("collection-item edge = %#v", itemEdge)
	}
	if got := bindingSummary(itemEdge.Bindings); !reflect.DeepEqual(got, []string{"child_id:row-property:id", "parent_id:frame-path:parent_id"}) {
		t.Fatalf("collection-item bindings = %#v", got)
	}

	ambiguous := viewWithOperation(t, descriptor, "listAmbiguousChildren")
	for _, edge := range descriptor.Edges {
		if edge.TargetViewID == ambiguous.ID && edge.Navigable {
			t.Fatalf("ambiguous view became navigable through %#v", edge)
		}
	}
	if !containsDiagnostic(descriptor.Diagnostics, "scoped view "+ambiguous.ID+" is not navigable") {
		t.Fatalf("ambiguous-view diagnostic absent: %#v", descriptor.Diagnostics)
	}

	itemOperations := operationIDs(childItem.OperationIDs)
	if !reflect.DeepEqual(itemOperations, []string{"archiveChild", "getChild", "patchChild", "streamChildEvents"}) {
		t.Fatalf("child item operations = %#v", itemOperations)
	}
	stream := operationByID(t, descriptor, "streamChildEvents")
	if !stream.Response.Stream || !reflect.DeepEqual(stream.Capabilities, []string{"stream"}) {
		t.Fatalf("stream capability = %#v", stream)
	}
	patch := operationByID(t, descriptor, "patchChild")
	if patch.RequestBody == nil || !patch.RequestBody.Required || len(patch.RequestBody.Fields) != 1 || patch.RequestBody.Fields[0].Name != "name" {
		t.Fatalf("request fields did not exclude read-only values: %#v", patch.RequestBody)
	}
	if !reflect.DeepEqual(patch.RequestBody.Fields[0].Enum, []any{"new", "archived"}) || patch.RequestBody.Fields[0].Default != "new" {
		t.Fatalf("request field choices/default = %#v", patch.RequestBody.Fields[0])
	}
	archive := operationByID(t, descriptor, "archiveChild")
	if gap := requiredProjectionInputs(*archive); !reflect.DeepEqual(gap, []string{"header:X-Reason", "query:notify"}) {
		t.Fatalf("action required inputs = %#v", gap)
	}
	if parameter := operationParameter(archive, "notify"); parameter == nil || parameter.Style != "form" || !parameter.Explode || !parameter.AllowReserved {
		t.Fatalf("query serialization metadata was not preserved: %#v", parameter)
	} else if parameter.Default != false {
		t.Fatalf("query default was not preserved: %#v", parameter)
	}
	if archive.Presentation.Label != "Archive child" || archive.Presentation.Hotkey != "x" || archive.Presentation.Confirmation == nil || archive.Presentation.Confirmation.Title != "Confirm archive" || archive.Presentation.Confirmation.Destructive {
		t.Fatalf("action presentation metadata = %#v", archive.Presentation)
	}
}

func TestDeleteActionAlwaysProjectsDestructiveConfirmation(t *testing.T) {
	document, err := ir.Load("testdata/navigation.yaml", ir.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range document.Operations {
		if operation.ID == "archiveChild" {
			operation.Method = http.MethodDelete
			delete(operation.Extensions, tuiExtension)
		}
	}
	descriptor, err := projectDocument(document)
	if err != nil {
		t.Fatal(err)
	}
	archive := operationByID(t, descriptor, "archiveChild")
	if archive.Presentation.Confirmation == nil || !archive.Presentation.Confirmation.Destructive || archive.Presentation.Confirmation.Title != "Delete" || archive.Presentation.Confirmation.Message == "" || strings.Contains(archive.Presentation.Confirmation.Message, "Run ") {
		t.Fatalf("delete confirmation = %#v", archive.Presentation.Confirmation)
	}
}

func TestConflictingActionHotkeysReportBothOperations(t *testing.T) {
	original, err := os.ReadFile("testdata/navigation.yaml")
	if err != nil {
		t.Fatal(err)
	}
	contents := strings.Replace(string(original), "      operationId: patchChild", "      operationId: patchChild\n      x-trex-tui: {hotkey: x}", 1)
	path := filepath.Join(t.TempDir(), "conflict.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	document, err := ir.Load(path, ir.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = projectDocument(document)
	if err == nil || !strings.Contains(err.Error(), "archiveChild") || !strings.Contains(err.Error(), "patchChild") || !strings.Contains(err.Error(), "hotkey \"x\" conflicts") || !strings.Contains(err.Error(), path+"#/") {
		t.Fatalf("hotkey conflict diagnostic = %v", err)
	}
}

func TestCollectionAndHighlightedItemHotkeysConflict(t *testing.T) {
	original, err := os.ReadFile("testdata/navigation.yaml")
	if err != nil {
		t.Fatal(err)
	}
	needle := "    get:\n      operationId: listChildren"
	replacement := `    post:
      operationId: createChild
      x-trex-tui: {hotkey: x}
      responses:
        "201": {description: child, content: {application/json: {schema: {$ref: "#/components/schemas/Child"}}}}
    get:
      operationId: listChildren`
	contents := strings.Replace(string(original), needle, replacement, 1)
	if contents == string(original) {
		t.Fatal("collection action fixture replacement did not match")
	}
	path := filepath.Join(t.TempDir(), "collection-item-conflict.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	document, err := ir.Load(path, ir.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = projectDocument(document)
	if err == nil || !strings.Contains(err.Error(), "createChild") || !strings.Contains(err.Error(), "archiveChild") || !strings.Contains(err.Error(), "hotkey \"x\" conflicts") || !strings.Contains(err.Error(), path+"#/") {
		t.Fatalf("collection/item hotkey conflict diagnostic = %v", err)
	}
}

func TestSharedIRConformanceFixtureProjection(t *testing.T) {
	document, err := ir.Load(filepath.Join("..", "openapi-ir", "testdata", "conformance", "openapi.yaml"), ir.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	descriptor, projectionErr := projectDocument(document)
	if projectionErr == nil || !strings.Contains(projectionErr.Error(), "no supported HTTP bearer alternative") || !strings.Contains(projectionErr.Error(), "oauth") {
		t.Fatalf("shared fixture projection error = %v, want unsupported OAuth diagnostic", projectionErr)
	}

	list := operationByID(t, descriptor, "listWidgets")
	if !list.Security.None || !reflect.DeepEqual(list.Capabilities, []string{"list"}) || list.Response.ItemsPointer != "/items" {
		t.Fatalf("shared list projection = %#v", list)
	}
	path, err := tui.BuildPath(*list, map[string]any{})
	if err != nil || path != "/widgets" {
		t.Fatalf("shared list path = %q, err=%v", path, err)
	}
	create := operationByID(t, descriptor, "createWidget")
	if create.RequestBody == nil || !create.RequestBody.Required || !reflect.DeepEqual(create.Capabilities, []string{"create"}) {
		t.Fatalf("shared create projection = %#v", create)
	}
	collection := viewWithOperation(t, descriptor, "listWidgets")
	item := viewWithOperation(t, descriptor, "getWidget")
	edge := edgeBetween(t, descriptor, collection.ID, item.ID)
	if edge.Provenance != "explicit-link" || !edge.Navigable || !reflect.DeepEqual(bindingSummary(edge.Bindings), []string{"widget_id:runtime-expression:$response.body#/items/0/id"}) {
		t.Fatalf("shared relationship projection = %#v", edge)
	}
}

func TestPartialCapabilitiesProjectOnlyDocumentedControls(t *testing.T) {
	descriptor := loadProjectedFixture(t, "testdata/partial-capabilities.yaml")
	collection := viewWithOperation(t, descriptor, "listRecords")
	item := viewWithOperation(t, descriptor, "patchRecord")
	if !reflect.DeepEqual(collection.Capabilities, []string{"list"}) {
		t.Fatalf("collection capabilities = %v, want list only", collection.Capabilities)
	}
	wantItem := []string{"stream", "update"}
	if !reflect.DeepEqual(item.Capabilities, wantItem) {
		t.Fatalf("item capabilities = %v, want %v", item.Capabilities, wantItem)
	}
	if got := operationIDs(item.OperationIDs); !reflect.DeepEqual(got, []string{"patchRecord", "streamRecordEvents"}) {
		t.Fatalf("item controls = %v, want documented patch and stream only", got)
	}
}

func TestMappedBindingGrammar(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		expression any
		kind       string
		source     string
	}{
		{name: "url", expression: "$url", kind: "runtime-expression", source: "$url"},
		{name: "method", expression: "$method", kind: "runtime-expression", source: "$method"},
		{name: "status code", expression: "$statusCode", kind: "runtime-expression", source: "$statusCode"},
		{name: "request path", expression: "$request.path.project_id", kind: "runtime-expression", source: "$request.path.project_id"},
		{name: "request query", expression: "$request.query.filter", kind: "runtime-expression", source: "$request.query.filter"},
		{name: "request header", expression: "$request.header.X-Tenant", kind: "runtime-expression", source: "$request.header.X-Tenant"},
		{name: "complete request body", expression: "$request.body", kind: "runtime-expression", source: "$request.body"},
		{name: "request body", expression: "$request.body#/parent/id", kind: "runtime-expression", source: "$request.body#/parent/id"},
		{name: "response header", expression: "$response.header.Location", kind: "runtime-expression", source: "$response.header.Location"},
		{name: "complete response body", expression: "$response.body", kind: "runtime-expression", source: "$response.body"},
		{name: "response body", expression: "$response.body#/id", kind: "runtime-expression", source: "$response.body#/id"},
		{name: "string literal", expression: "fixed", kind: "literal", source: "fixed"},
		{name: "numeric literal", expression: 7, kind: "literal", source: "7"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			binding, err := mappedBinding("target", testCase.expression)
			if err != nil {
				t.Fatal(err)
			}
			if binding.SourceKind != testCase.kind || binding.Source != testCase.source {
				t.Fatalf("binding = %#v", binding)
			}
		})
	}
	for _, invalid := range []any{"$request.query.", "$request.body/not-a-pointer", "$response.header.", nil} {
		if binding, err := mappedBinding("target", invalid); err == nil {
			t.Fatalf("invalid expression %#v produced binding %#v", invalid, binding)
		}
	}
}

func TestOptionalSecurityAlternativeIsPreserved(t *testing.T) {
	document := &ir.Document{
		SecuritySchemes: []*ir.SecurityScheme{{Name: "Bearer", Type: "http", Scheme: "bearer"}},
	}
	operation := &ir.Operation{Security: ir.OperationSecurity{
		State: ir.SecurityOverride,
		Requirements: []ir.SecurityRequirement{
			{Schemes: []ir.SecuritySchemeUse{}},
			{Schemes: []ir.SecuritySchemeUse{{Name: "Bearer"}}},
		},
	}}
	projector := &projection{document: document}
	security := projector.security(operation)
	if security.None || len(security.Requirements) != 2 || len(security.Requirements[0].Schemes) != 0 || !reflect.DeepEqual(security.Requirements[1].Schemes, []string{"Bearer"}) {
		t.Fatalf("optional security projection = %#v", security)
	}
	if len(projector.fatal) != 0 {
		t.Fatalf("optional anonymous alternative produced fatal diagnostics: %v", projector.fatal)
	}
}

func TestInvalidPresentationExtensionsFailBeforeWriting(t *testing.T) {
	original, err := os.ReadFile("testdata/navigation.yaml")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, old, replacement, want string
	}{
		{name: "unknown field", old: "        label: Parents", replacement: "        unknown-field: nope\n        label: Parents", want: `unknown x-trex-tui field "unknown-field"`},
		{name: "invalid alias", old: "        aliases: [pa]", replacement: "        aliases: [Bad]", want: `alias "Bad" is invalid`},
		{name: "missing sort property", old: "        default-sort: name", replacement: "        default-sort: missing", want: `default-sort "missing" is not a readable scalar property`},
		{name: "terminal control label", old: "        label: Parents", replacement: "        label: \"bad\\u001b[31m\"", want: "terminal-safe string"},
		{name: "unsupported action visibility", old: "        hotkey: x", replacement: "        hotkey: x\n        visibility: hidden", want: `x-trex-tui field "visibility" is unsupported`},
		{name: "local action hotkey", old: "        hotkey: x", replacement: "        hotkey: a", want: `hotkey "a" conflicts with the shared keybinding registry`},
		{name: "raw resource hotkey", old: "        hotkey: x", replacement: "        hotkey: r", want: `hotkey "r" conflicts with the shared keybinding registry`},
		{name: "global control hotkey", old: "        hotkey: x", replacement: "        hotkey: ctrl-c", want: `hotkey "ctrl-c" conflicts with the shared keybinding registry`},
		{name: "unsafe confirmation", old: "          message: Archive the selected child?", replacement: "          message: \"bad\\u001b[31m\"", want: "message must be a non-empty terminal-safe string"},
		{name: "incomplete explicit binding", old: "            children:\n              operationId: listChildren\n              parameters: {parent_id: \"$response.body#/id\"}", replacement: "            children:\n              operationId: listAmbiguousChildren", want: "unsatisfied path parameters: organization_id, project_id"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			contents := strings.Replace(string(original), testCase.old, testCase.replacement, 1)
			if contents == string(original) {
				t.Fatalf("fixture replacement %q did not match", testCase.old)
			}
			directory := t.TempDir()
			specPath := filepath.Join(directory, "invalid.yaml")
			if err := os.WriteFile(specPath, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			document, err := ir.Load(specPath, ir.LoadOptions{})
			if err != nil {
				t.Fatal(err)
			}
			_, err = projectDocument(document)
			if err == nil || !strings.Contains(err.Error(), testCase.want) || !strings.Contains(err.Error(), specPath+"#/") {
				t.Fatalf("diagnostic = %v, want location and %q", err, testCase.want)
			}

			output := filepath.Join(directory, "output")
			err = generate(generateOptions{SpecPath: specPath, OutDir: output})
			if err == nil {
				t.Fatal("invalid projection generated output")
			}
			if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
				t.Fatalf("invalid generation left output behind: %v", statErr)
			}
		})
	}
}

func TestUnsupportedRequiredSecurityFailsProjection(t *testing.T) {
	original, err := os.ReadFile("testdata/navigation.yaml")
	if err != nil {
		t.Fatal(err)
	}
	contents := strings.Replace(string(original),
		"Bearer: {type: http, scheme: bearer, bearerFormat: JWT}",
		"Bearer: {type: apiKey, in: header, name: X-API-Key}", 1)
	specPath := filepath.Join(t.TempDir(), "unsupported-security.yaml")
	if err := os.WriteFile(specPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	document, err := ir.Load(specPath, ir.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = projectDocument(document)
	if err == nil || !strings.Contains(err.Error(), "operation listParents") || !strings.Contains(err.Error(), "declared: Bearer") {
		t.Fatalf("unsupported-security diagnostic = %v", err)
	}
}

func loadProjectedFixture(t *testing.T, path string) tui.Descriptor {
	t.Helper()
	document, err := ir.Load(path, ir.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := projectDocument(document)
	if err != nil {
		t.Fatal(err)
	}
	return descriptor
}

func viewWithOperation(t *testing.T, descriptor tui.Descriptor, operationID string) *tui.View {
	t.Helper()
	for index := range descriptor.Views {
		for _, candidate := range descriptor.Views[index].OperationIDs {
			if candidate == operationID {
				return &descriptor.Views[index]
			}
		}
	}
	t.Fatalf("view for operation %s not found", operationID)
	return nil
}

func operationByID(t *testing.T, descriptor tui.Descriptor, id string) *tui.Operation {
	t.Helper()
	operation := descriptor.Operation(id)
	if operation == nil {
		t.Fatalf("operation %s not found", id)
	}
	return operation
}

func operationParameter(operation *tui.Operation, name string) *tui.Parameter {
	for index := range operation.Parameters {
		if operation.Parameters[index].Name == name {
			return &operation.Parameters[index]
		}
	}
	return nil
}

func edgeBetween(t *testing.T, descriptor tui.Descriptor, source, target string) *tui.Edge {
	t.Helper()
	for index := range descriptor.Edges {
		edge := &descriptor.Edges[index]
		if edge.SourceViewID == source && edge.TargetViewID == target {
			return edge
		}
	}
	t.Fatalf("edge %s -> %s not found", source, target)
	return nil
}

func assertExplicitRuntimeEdge(t *testing.T, descriptor tui.Descriptor, source, target, sourceOperation, expression string) {
	t.Helper()
	edge := edgeBetween(t, descriptor, source, target)
	if edge.Provenance != "explicit-link" || edge.SourceOperationID != sourceOperation || edge.TargetOperationID != "listChildren" || !edge.Navigable {
		t.Fatalf("explicit edge = %#v", edge)
	}
	if got := bindingSummary(edge.Bindings); !reflect.DeepEqual(got, []string{"parent_id:runtime-expression:" + expression}) {
		t.Fatalf("explicit bindings = %#v", got)
	}
}

func bindingSummary(bindings []tui.Binding) []string {
	result := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		result = append(result, binding.Target+":"+binding.SourceKind+":"+binding.Source)
	}
	sort.Strings(result)
	return result
}

func columnSummary(columns []tui.Column) []string {
	result := make([]string, 0, len(columns))
	for _, column := range columns {
		result = append(result, column.Property+":"+column.Label+":"+strconv.Itoa(column.Priority))
	}
	return result
}

func operationIDs(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func containsDiagnostic(diagnostics []string, want string) bool {
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic, want) {
			return true
		}
	}
	return false
}

func requiredProjectionInputs(operation tui.Operation) []string {
	var result []string
	for _, parameter := range operation.Parameters {
		if parameter.Required && parameter.In != "path" {
			result = append(result, parameter.In+":"+parameter.Name)
		}
	}
	sort.Strings(result)
	return result
}
