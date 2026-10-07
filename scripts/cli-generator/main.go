package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"unicode"

	ir "github.com/openshift-online/rh-trex-ai/scripts/openapi-ir"
)

func main() {
	specPath := flag.String("spec", "", "path to openapi.yaml")
	outDir := flag.String("out", "", "output directory for the CLI project")
	binaryName := flag.String("binary", "", "name of the CLI binary (e.g. trex-cli)")
	projectName := flag.String("project", "", "project name (e.g. rh-trex-ai)")
	apiPrefix := flag.String("api-prefix", "", "API path prefix (e.g. /api/rh-trex-ai/v1)")
	cliModule := flag.String("module", "", "Go module path for the CLI (e.g. github.com/myorg/myproject-cli)")
	configName := flag.String("config-name", "", "name used for the config file and its environment variable, e.g. hypershell gives ~/.hypershell.json and HYPERSHELL_CONFIG (default: the binary name)")
	oidcClientID := flag.String("oidc-client-id", "", "default OpenID Connect client ID for 'login --issuer-url' (default: the binary name)")
	flag.Parse()

	if *specPath == "" || *outDir == "" {
		log.Fatal("--spec and --out are required")
	}

	if *apiPrefix == "" {
		*apiPrefix = inferAPIPrefix(*specPath)
	}

	if *projectName == "" {
		parts := strings.Split(*apiPrefix, "/")
		for _, p := range parts {
			if p != "" && p != "api" && p != "v1" && p != "v2" {
				*projectName = p
				break
			}
		}
	}

	if *binaryName == "" {
		*binaryName = strings.ReplaceAll(*projectName, "-", "")
		*binaryName = strings.ReplaceAll(*binaryName, "_", "")
	}

	if *cliModule == "" {
		*cliModule = "github.com/example/" + *binaryName + "-cli"
	}

	resources, err := parseResources(*specPath, *apiPrefix)
	if err != nil {
		log.Fatalf("parse resources: %v", err)
	}

	fmt.Printf("Generating CLI '%s' for %d resources\n", *binaryName, len(resources))
	for _, r := range resources {
		fmt.Printf("  %s (%s): %d writable fields\n", r.Name, r.PathSegment, len(r.WritableFields))
	}

	data := cliData{
		Binary:    *binaryName,
		Project:   *projectName,
		APIPrefix: *apiPrefix,
		Module:    *cliModule,
		Resources: resources,

		OIDCClientID: *oidcClientID,
		ConfigName:   *configName,
	}

	if err := generateCLI(data, *outDir); err != nil {
		log.Fatalf("generate CLI: %v", err)
	}

	fmt.Printf("CLI generated in %s\n", *outDir)
}

type cliResource struct {
	Name           string
	NameLower      string
	Plural         string
	PluralLower    string
	PathSegment    string
	DefaultColumns string
	WritableFields []cliField
	KindListName   string
	// DeleteEnabled and UpdateEnabled are set only when the item view of the
	// resource declares the corresponding operation in the OpenAPI document.
	DeleteEnabled bool
	UpdateEnabled bool
	// UpdateMethod is PATCH when the item view declares it, otherwise PUT.
	UpdateMethod string
	// UpdateFields are the writable fields of the update request body.
	UpdateFields []cliField
}

type cliField struct {
	Name      string
	FlagName  string
	GoType    string
	FieldType string
}

type cliData struct {
	Binary    string
	Project   string
	APIPrefix string
	Module    string
	Resources []cliResource
	// OIDCClientID is the default for 'login --client-id'; empty means the binary name.
	OIDCClientID string
	// ConfigName names the config file and its environment variable; empty means the binary name.
	ConfigName string
}

var configNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ConfigFileName is the name the generated config file and directory use.
func (data cliData) ConfigFileName() string {
	if data.ConfigName != "" {
		return data.ConfigName
	}
	return data.Binary
}

// ConfigEnvVar is the environment variable that overrides the config file
// location: the config name upper-cased with '-' and '.' mapped to '_'.
func (data cliData) ConfigEnvVar() string {
	return strings.NewReplacer("-", "_", ".", "_").Replace(strings.ToUpper(data.ConfigFileName())) + "_CONFIG"
}

var oidcClientIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:@/-]+$`)

// DefaultClientID is the default OpenID Connect client ID of the generated login.
func (data cliData) DefaultClientID() string {
	if data.OIDCClientID != "" {
		return data.OIDCClientID
	}
	return data.Binary
}

// HasDelete reports whether any resource declares a delete operation.
func (data cliData) HasDelete() bool {
	for _, resource := range data.Resources {
		if resource.DeleteEnabled {
			return true
		}
	}
	return false
}

// HasUpdate reports whether any resource declares an update operation.
func (data cliData) HasUpdate() bool {
	for _, resource := range data.Resources {
		if resource.UpdateEnabled {
			return true
		}
	}
	return false
}

func parseResources(specPath, apiPrefix string) ([]cliResource, error) {
	document, err := ir.Load(specPath, ir.LoadOptions{})
	if err != nil {
		return nil, fmt.Errorf("load canonical OpenAPI IR: %w", err)
	}
	if err := document.ValidateProjectionNames(); err != nil {
		return nil, fmt.Errorf("validate CLI projection: %w", err)
	}

	var resources []cliResource
	for _, view := range cliPrimaryCollectionViews(document, apiPrefix) {
		schema := document.Schema(view.SchemaRef)
		if schema == nil || schema.Name == "" {
			continue
		}
		fields := extractWritableFields(document, schema)
		columns := buildDefaultColumns(document, schema)
		nameLower := toLowerFirst(schema.Name)
		pluralName := pluralizeName(schema.Name)
		pluralLower := toLowerFirst(pluralName)

		resource := cliResource{
			Name:           schema.Name,
			NameLower:      nameLower,
			Plural:         pluralName,
			PluralLower:    pluralLower,
			PathSegment:    cliLastSegment(view.Path),
			DefaultColumns: columns,
			WritableFields: fields,
			KindListName:   schema.Name + "List",
		}
		applyItemOperations(document, view, &resource)
		resources = append(resources, resource)
	}

	sort.Slice(resources, func(i, j int) bool {
		return resources[i].Name < resources[j].Name
	})

	return resources, nil
}

// applyItemOperations enables delete and update for a resource when its item
// view (the collection path plus one trailing path parameter) declares them.
func applyItemOperations(document *ir.Document, collection *ir.ResourceView, resource *cliResource) {
	prefix := strings.TrimSuffix(collection.Path, "/") + "/"
	var deleteOperation, patchOperation, putOperation *ir.Operation
	for _, view := range document.ResourceViews {
		if view.Kind != ir.ResourceItem || view.SchemaRef != collection.SchemaRef || !strings.HasPrefix(view.Path, prefix) {
			continue
		}
		identifier := strings.TrimPrefix(view.Path, prefix)
		if !strings.HasPrefix(identifier, "{") || !strings.HasSuffix(identifier, "}") || strings.Contains(identifier, "/") {
			continue
		}
		for _, operationID := range view.OperationIDs {
			operation := document.Operation(operationID)
			if operation == nil {
				continue
			}
			switch {
			case operation.Method == "DELETE" && operation.Capabilities.Has(ir.CapabilityDelete):
				deleteOperation = operation
			case operation.Method == "PATCH" && operation.Capabilities.Has(ir.CapabilityUpdate):
				patchOperation = operation
			case operation.Method == "PUT" && operation.Capabilities.Has(ir.CapabilityUpdate):
				putOperation = operation
			}
		}
	}
	resource.DeleteEnabled = deleteOperation != nil
	update := patchOperation
	resource.UpdateMethod = "PATCH"
	if update == nil && putOperation != nil {
		update, resource.UpdateMethod = putOperation, "PUT"
	}
	if update == nil {
		return
	}
	resource.UpdateEnabled = true
	resource.UpdateFields = resource.WritableFields
	if update.RequestBody != nil {
		for _, content := range update.RequestBody.Content {
			if content.Schema == nil {
				continue
			}
			if body := document.Schema(content.Schema.Ref); body != nil {
				if fields := extractWritableFields(document, body); len(fields) > 0 {
					resource.UpdateFields = fields
				}
				break
			}
		}
	}
}

func extractWritableFields(document *ir.Document, schema *ir.Schema) []cliField {
	var fields []cliField
	objRefFields := map[string]bool{
		"id": true, "kind": true, "href": true, "created_at": true, "updated_at": true,
	}
	properties := cliTargetProperties(document, schema)
	for propName, property := range properties {
		if objRefFields[propName] {
			continue
		}
		if property.ReadOnly {
			continue
		}
		propType, _ := cliSchemaType(document.Schema(property.Schema.Ref))
		flagName := strings.ReplaceAll(propName, "_", "-")
		goType := "string"
		switch propType {
		case "integer":
			goType = "int"
		case "boolean":
			goType = "bool"
		case "number":
			goType = "float64"
		}

		fields = append(fields, cliField{
			Name:      propName,
			FlagName:  flagName,
			GoType:    goType,
			FieldType: propType,
		})
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
	return fields
}

func buildDefaultColumns(document *ir.Document, schema *ir.Schema) string {
	var fieldNames []string
	for name := range cliTargetProperties(document, schema) {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)

	cols := []string{"id"}
	for _, f := range fieldNames {
		if f == "id" || f == "kind" || f == "href" {
			continue
		}
		cols = append(cols, f)
		if len(cols) >= 5 {
			break
		}
	}
	cols = append(cols, "created_at")
	return strings.Join(cols, ", ")
}

func cliTargetProperties(document *ir.Document, schema *ir.Schema) map[string]*ir.Property {
	if len(schema.AllOf) == 0 {
		return schema.Properties
	}
	result := make(map[string]*ir.Property)
	for _, reference := range schema.AllOf {
		part := document.Schema(reference.Ref)
		if part == nil || part.Name != "" {
			continue
		}
		for name, property := range part.Properties {
			result[name] = property
		}
	}
	return result
}

func cliSchemaType(schema *ir.Schema) (string, string) {
	if schema == nil || len(schema.Types) == 0 {
		return "", ""
	}
	return schema.Types[0], schema.Format
}

func cliPrimaryCollectionViews(document *ir.Document, apiPrefix string) []*ir.ResourceView {
	bySchema := make(map[string]*ir.ResourceView)
	for _, view := range document.ResourceViews {
		if view.Kind != ir.ResourceCollection || !view.Capabilities.Has(ir.CapabilityList) {
			continue
		}
		remainder := strings.TrimPrefix(view.Path, strings.TrimSuffix(apiPrefix, "/")+"/")
		if remainder == view.Path || remainder == "" || strings.Contains(remainder, "/") {
			continue
		}
		bySchema[view.SchemaRef] = view
	}
	result := make([]*ir.ResourceView, 0, len(bySchema))
	for _, view := range bySchema {
		result = append(result, view)
	}
	return result
}

func cliLastSegment(path string) string {
	path = strings.TrimSuffix(path, "/")
	if index := strings.LastIndexByte(path, '/'); index >= 0 {
		path = path[index+1:]
	}
	return strings.Trim(path, "{}")
}

// warningOutput receives generator warnings; tests replace it.
var warningOutput io.Writer = os.Stderr

// warnIfMainSkipsGeneratedCommands reports a hand-maintained main.go that does
// not call addGeneratedCommands: it is never overwritten, so without the call
// newly generated commands would exist but stay unreachable.
func warnIfMainSkipsGeneratedCommands(mainPath string) {
	source, err := os.ReadFile(mainPath)
	if err != nil || strings.Contains(string(source), "addGeneratedCommands(") {
		return
	}
	_, _ = fmt.Fprintf(warningOutput, "warning: %s is hand-maintained and was not regenerated, and it does not call addGeneratedCommands(root), "+
		"so newly generated commands are not registered. Replace its registrations of generated commands with a call to addGeneratedCommands(root).\n", mainPath)
}

func generateCLI(data cliData, outDir string) error {
	if data.ConfigName != "" && (!configNamePattern.MatchString(data.ConfigName) || strings.Contains(data.ConfigName, "..")) {
		return fmt.Errorf("invalid config name %q: use letters, digits and . _ - and start with a letter or digit", data.ConfigName)
	}
	if !oidcClientIDPattern.MatchString(data.DefaultClientID()) {
		return fmt.Errorf("invalid OpenID Connect client ID %q: use letters, digits and . _ : @ / -", data.DefaultClientID())
	}
	tmplDir := filepath.Join(getTemplateDir())

	type tmplMapping struct {
		tmplPath string
		outPath  string
		resource *cliResource
		// ifMissing leaves an existing file untouched so hand-maintained edits survive regeneration.
		ifMissing bool
	}

	var mappings []tmplMapping

	// Bootstrap-only files (written when absent, never overwritten):
	//   cmd/<binary>/main.go - registers both generated and hand-authored commands
	//   go.mod               - managed by go mod tidy; hand-authored cmds may add deps
	mappings = append(mappings,
		tmplMapping{"cmd/main.go.tmpl", filepath.Join("cmd", data.Binary, "main.go"), nil, true},
		tmplMapping{"gomod.tmpl", "go.mod", nil, true},
	)

	mappings = append(mappings,
		tmplMapping{"cmd/generated_commands.go.tmpl", filepath.Join("cmd", data.Binary, "generated_commands.go"), nil, false},
		tmplMapping{"cmd/login.go.tmpl", filepath.Join("cmd", data.Binary, "login", "cmd.go"), nil, false},
		tmplMapping{"cmd/logout.go.tmpl", filepath.Join("cmd", data.Binary, "logout", "cmd.go"), nil, false},
		tmplMapping{"cmd/whoami.go.tmpl", filepath.Join("cmd", data.Binary, "whoami", "cmd.go"), nil, false},
		tmplMapping{"cmd/version.go.tmpl", filepath.Join("cmd", data.Binary, "version", "cmd.go"), nil, false},
		tmplMapping{"cmd/completion.go.tmpl", filepath.Join("cmd", data.Binary, "completion", "cmd.go"), nil, false},
		tmplMapping{"cmd/config.go.tmpl", filepath.Join("cmd", data.Binary, "config", "cmd.go"), nil, false},
		tmplMapping{"cmd/list.go.tmpl", filepath.Join("cmd", data.Binary, "list", "cmd.go"), nil, false},
		tmplMapping{"cmd/get.go.tmpl", filepath.Join("cmd", data.Binary, "get", "cmd.go"), nil, false},
		tmplMapping{"cmd/create.go.tmpl", filepath.Join("cmd", data.Binary, "create", "cmd.go"), nil, false},
		tmplMapping{"pkg/config.go.tmpl", filepath.Join("pkg", "config", "config.go"), nil, false},
		tmplMapping{"pkg/token.go.tmpl", filepath.Join("pkg", "config", "token.go"), nil, false},
		tmplMapping{"pkg/connection.go.tmpl", filepath.Join("pkg", "connection", "connection.go"), nil, false},
		tmplMapping{"pkg/oidc.go.tmpl", filepath.Join("pkg", "oidc", "oidc.go"), nil, false},
		tmplMapping{"pkg/dump.go.tmpl", filepath.Join("pkg", "dump", "dump.go"), nil, false},
		tmplMapping{"pkg/printer.go.tmpl", filepath.Join("pkg", "output", "printer.go"), nil, false},
		tmplMapping{"pkg/table.go.tmpl", filepath.Join("pkg", "output", "table.go"), nil, false},
		tmplMapping{"pkg/terminal.go.tmpl", filepath.Join("pkg", "output", "terminal.go"), nil, false},
		tmplMapping{"pkg/arguments.go.tmpl", filepath.Join("pkg", "arguments", "arguments.go"), nil, false},
		tmplMapping{"pkg/urls.go.tmpl", filepath.Join("pkg", "urls", "urls.go"), nil, false},
		tmplMapping{"pkg/info.go.tmpl", filepath.Join("pkg", "info", "info.go"), nil, false},
	)

	if data.HasDelete() {
		mappings = append(mappings, tmplMapping{"pkg/confirm.go.tmpl", filepath.Join("pkg", "confirm", "confirm.go"), nil, false})
		mappings = append(mappings, tmplMapping{"cmd/delete.go.tmpl", filepath.Join("cmd", data.Binary, "delete", "cmd.go"), nil, false})
	}
	if data.HasUpdate() {
		mappings = append(mappings, tmplMapping{"cmd/update.go.tmpl", filepath.Join("cmd", data.Binary, "update", "cmd.go"), nil, false})
	}

	for i := range data.Resources {
		r := &data.Resources[i]
		mappings = append(mappings,
			tmplMapping{"cmd/list_resource.go.tmpl", filepath.Join("cmd", data.Binary, "list", r.PluralLower, "cmd.go"), r, false},
			tmplMapping{"cmd/get_resource.go.tmpl", filepath.Join("cmd", data.Binary, "get", r.NameLower, "cmd.go"), r, false},
			tmplMapping{"cmd/create_resource.go.tmpl", filepath.Join("cmd", data.Binary, "create", r.NameLower, "cmd.go"), r, false},
		)
		if r.DeleteEnabled {
			mappings = append(mappings, tmplMapping{"cmd/delete_resource.go.tmpl", filepath.Join("cmd", data.Binary, "delete", r.NameLower, "cmd.go"), r, false})
		}
		if r.UpdateEnabled {
			mappings = append(mappings, tmplMapping{"cmd/update_resource.go.tmpl", filepath.Join("cmd", data.Binary, "update", r.NameLower, "cmd.go"), r, false})
		}
	}

	for _, m := range mappings {
		tmplPath := filepath.Join(tmplDir, m.tmplPath)
		outPath, err := ir.SafeJoin(outDir, m.outPath)
		if err != nil {
			return fmt.Errorf("resolve output %s: %w", m.outPath, err)
		}

		if m.ifMissing {
			if _, err := os.Stat(outPath); err == nil {
				if m.tmplPath == "cmd/main.go.tmpl" {
					warnIfMainSkipsGeneratedCommands(outPath)
				}
				continue
			}
		}

		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(outPath), err)
		}

		tmpl, err := loadCLITemplate(tmplPath)
		if err != nil {
			return fmt.Errorf("load template %s: %w", m.tmplPath, err)
		}

		td := struct {
			cliData
			Resource cliResource
		}{cliData: data}

		if m.resource != nil {
			td.Resource = *m.resource
		}

		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("create %s: %w", outPath, err)
		}
		if err := tmpl.Execute(f, td); err != nil {
			f.Close()
			return fmt.Errorf("execute template %s: %w", m.tmplPath, err)
		}
		f.Close()
	}

	return nil
}

func loadCLITemplate(path string) (*template.Template, error) {
	funcMap := template.FuncMap{
		"lower":      strings.ToLower,
		"upper":      strings.ToUpper,
		"title":      strings.Title,
		"snakeCase":  toSnakeCase,
		"kebabCase":  toKebabCase,
		"camelCase":  toCamelCase,
		"pascalCase": toPascalCase,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New(filepath.Base(path)).Funcs(funcMap).Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return tmpl, nil
}

func getTemplateDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "templates"
	}
	dir := filepath.Dir(exe)
	tmplDir := filepath.Join(dir, "templates")
	if _, err := os.Stat(tmplDir); err == nil {
		return tmplDir
	}
	return "templates"
}

func inferAPIPrefix(specPath string) string {
	document, err := ir.Load(specPath, ir.LoadOptions{})
	if err != nil {
		return "/api/v1"
	}
	for _, operation := range document.Operations {
		if strings.HasPrefix(operation.Path, "/api/") {
			parts := strings.Split(operation.Path, "/")
			if len(parts) >= 4 {
				return "/" + parts[1] + "/" + parts[2] + "/" + parts[3]
			}
		}
	}
	return "/api/v1"
}

func pluralizeName(name string) string {
	if strings.HasSuffix(name, "Settings") || strings.HasSuffix(name, "Data") {
		return name
	}
	if strings.HasSuffix(name, "s") {
		return name + "es"
	}
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, "y") {
		lastChar := lower[len(lower)-2]
		if lastChar != 'a' && lastChar != 'e' && lastChar != 'i' && lastChar != 'o' && lastChar != 'u' {
			return name[:len(name)-1] + "ies"
		}
	}
	return name + "s"
}

func toLowerFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			result.WriteRune('_')
		}
		result.WriteRune(unicode.ToLower(r))
	}
	return result.String()
}

func toKebabCase(s string) string {
	return strings.ReplaceAll(toSnakeCase(s), "_", "-")
}

func toCamelCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' })
	if len(parts) == 0 {
		return s
	}
	var result strings.Builder
	result.WriteString(strings.ToLower(parts[0]))
	for _, part := range parts[1:] {
		if len(part) > 0 {
			result.WriteString(strings.ToUpper(string(part[0])) + part[1:])
		}
	}
	return result.String()
}

func toPascalCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' })
	var result strings.Builder
	for _, part := range parts {
		if len(part) > 0 {
			result.WriteString(strings.ToUpper(string(part[0])) + part[1:])
		}
	}
	return result.String()
}
