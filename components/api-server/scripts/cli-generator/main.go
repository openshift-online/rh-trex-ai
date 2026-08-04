package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"unicode"

	"gopkg.in/yaml.v3"
)

func main() {
	specPath := flag.String("spec", "", "path to openapi.yaml")
	outDir := flag.String("out", "", "output directory for the CLI project")
	binaryName := flag.String("binary", "", "name of the CLI binary (e.g. trex-cli)")
	projectName := flag.String("project", "", "project name (e.g. rh-trex-ai)")
	apiPrefix := flag.String("api-prefix", "", "API path prefix (e.g. /api/rh-trex-ai/v1)")
	cliModule := flag.String("module", "", "Go module path for the CLI (e.g. github.com/myorg/myproject-cli)")
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
}

func parseResources(specPath, apiPrefix string) ([]cliResource, error) {
	mainData, err := os.ReadFile(specPath)
	if err != nil {
		return nil, err
	}

	var doc struct {
		Paths      map[string]interface{} `yaml:"paths"`
		Components struct {
			Schemas map[string]interface{} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(mainData, &doc); err != nil {
		return nil, err
	}

	specDir := filepath.Dir(specPath)
	var resources []cliResource

	for schemaName, schemaVal := range doc.Components.Schemas {
		if strings.HasSuffix(schemaName, "List") || strings.HasSuffix(schemaName, "PatchRequest") ||
			strings.HasSuffix(schemaName, "StatusPatchRequest") {
			continue
		}
		if schemaName == "ObjectReference" || schemaName == "List" || schemaName == "Error" {
			continue
		}

		refMap, ok := schemaVal.(map[string]interface{})
		if !ok {
			continue
		}
		refStr, ok := refMap["$ref"].(string)
		if !ok {
			continue
		}
		parts := strings.SplitN(refStr, "#", 2)
		if len(parts) != 2 {
			continue
		}

		subFile := parts[0]
		subPath := filepath.Join(specDir, subFile)
		subData, err := os.ReadFile(subPath)
		if err != nil {
			continue
		}

		var subDoc struct {
			Components struct {
				Schemas map[string]interface{} `yaml:"schemas"`
			} `yaml:"components"`
		}
		if err := yaml.Unmarshal(subData, &subDoc); err != nil {
			continue
		}

		pathSegment := inferPathSegmentFromPaths(doc.Paths, apiPrefix, schemaName)
		fields := extractWritableFields(subDoc.Components.Schemas, schemaName)
		columns := buildDefaultColumns(subDoc.Components.Schemas, schemaName)

		nameLower := toLowerFirst(schemaName)
		pluralName := pluralizeName(schemaName)
		pluralLower := toLowerFirst(pluralName)

		resources = append(resources, cliResource{
			Name:           schemaName,
			NameLower:      nameLower,
			Plural:         pluralName,
			PluralLower:    pluralLower,
			PathSegment:    pathSegment,
			DefaultColumns: columns,
			WritableFields: fields,
			KindListName:   schemaName + "List",
		})
	}

	sort.Slice(resources, func(i, j int) bool {
		return resources[i].Name < resources[j].Name
	})

	return resources, nil
}

func extractWritableFields(schemas map[string]interface{}, name string) []cliField {
	schema, ok := schemas[name]
	if !ok {
		return nil
	}

	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		return nil
	}

	var fields []cliField

	allOf, ok := schemaMap["allOf"]
	if ok {
		allOfList, ok := allOf.([]interface{})
		if ok {
			for _, item := range allOfList {
				itemMap, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				if _, hasRef := itemMap["$ref"]; hasRef {
					continue
				}
				fields = append(fields, extractPropsAsFields(itemMap)...)
			}
		}
	} else {
		fields = extractPropsAsFields(schemaMap)
	}

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Name < fields[j].Name
	})

	return fields
}

func extractPropsAsFields(m map[string]interface{}) []cliField {
	props, ok := m["properties"]
	if !ok {
		return nil
	}
	propsMap, ok := props.(map[string]interface{})
	if !ok {
		return nil
	}

	objRefFields := map[string]bool{
		"id": true, "kind": true, "href": true, "created_at": true, "updated_at": true,
	}

	var fields []cliField
	for propName, propVal := range propsMap {
		if objRefFields[propName] {
			continue
		}
		propMap, ok := propVal.(map[string]interface{})
		if !ok {
			continue
		}
		readOnly, _ := propMap["readOnly"].(bool)
		if readOnly {
			continue
		}

		propType, _ := propMap["type"].(string)
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
	return fields
}

func buildDefaultColumns(schemas map[string]interface{}, name string) string {
	schema, ok := schemas[name]
	if !ok {
		return "id, created_at"
	}
	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		return "id, created_at"
	}

	var fieldNames []string
	allOf, ok := schemaMap["allOf"]
	if ok {
		allOfList, ok := allOf.([]interface{})
		if ok {
			for _, item := range allOfList {
				itemMap, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				if _, hasRef := itemMap["$ref"]; hasRef {
					continue
				}
				props, ok := itemMap["properties"]
				if !ok {
					continue
				}
				propsMap, ok := props.(map[string]interface{})
				if !ok {
					continue
				}
				for k := range propsMap {
					fieldNames = append(fieldNames, k)
				}
			}
		}
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

func generateCLI(data cliData, outDir string) error {
	tmplDir := filepath.Join(getTemplateDir())

	type tmplMapping struct {
		tmplPath string
		outPath  string
	}

	var mappings []tmplMapping

	mappings = append(mappings,
		tmplMapping{"cmd/main.go.tmpl", filepath.Join("cmd", data.Binary, "main.go")},
		tmplMapping{"cmd/login.go.tmpl", filepath.Join("cmd", data.Binary, "login", "cmd.go")},
		tmplMapping{"cmd/logout.go.tmpl", filepath.Join("cmd", data.Binary, "logout", "cmd.go")},
		tmplMapping{"cmd/version.go.tmpl", filepath.Join("cmd", data.Binary, "version", "cmd.go")},
		tmplMapping{"cmd/completion.go.tmpl", filepath.Join("cmd", data.Binary, "completion", "cmd.go")},
		tmplMapping{"cmd/config.go.tmpl", filepath.Join("cmd", data.Binary, "config", "cmd.go")},
		tmplMapping{"cmd/list.go.tmpl", filepath.Join("cmd", data.Binary, "list", "cmd.go")},
		tmplMapping{"cmd/get.go.tmpl", filepath.Join("cmd", data.Binary, "get", "cmd.go")},
		tmplMapping{"cmd/create.go.tmpl", filepath.Join("cmd", data.Binary, "create", "cmd.go")},
		tmplMapping{"pkg/config.go.tmpl", filepath.Join("pkg", "config", "config.go")},
		tmplMapping{"pkg/token.go.tmpl", filepath.Join("pkg", "config", "token.go")},
		tmplMapping{"pkg/connection.go.tmpl", filepath.Join("pkg", "connection", "connection.go")},
		tmplMapping{"pkg/dump.go.tmpl", filepath.Join("pkg", "dump", "dump.go")},
		tmplMapping{"pkg/printer.go.tmpl", filepath.Join("pkg", "output", "printer.go")},
		tmplMapping{"pkg/table.go.tmpl", filepath.Join("pkg", "output", "table.go")},
		tmplMapping{"pkg/terminal.go.tmpl", filepath.Join("pkg", "output", "terminal.go")},
		tmplMapping{"pkg/arguments.go.tmpl", filepath.Join("pkg", "arguments", "arguments.go")},
		tmplMapping{"pkg/urls.go.tmpl", filepath.Join("pkg", "urls", "urls.go")},
		tmplMapping{"pkg/info.go.tmpl", filepath.Join("pkg", "info", "info.go")},
		tmplMapping{"gomod.tmpl", "go.mod"},
	)

	for _, r := range data.Resources {
		mappings = append(mappings,
			tmplMapping{"cmd/list_resource.go.tmpl", filepath.Join("cmd", data.Binary, "list", r.PluralLower, "cmd.go")},
			tmplMapping{"cmd/get_resource.go.tmpl", filepath.Join("cmd", data.Binary, "get", r.NameLower, "cmd.go")},
			tmplMapping{"cmd/create_resource.go.tmpl", filepath.Join("cmd", data.Binary, "create", r.NameLower, "cmd.go")},
		)
	}

	for _, m := range mappings {
		tmplPath := filepath.Join(tmplDir, m.tmplPath)
		outPath := filepath.Join(outDir, m.outPath)

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

		if strings.Contains(m.tmplPath, "_resource") {
			for _, r := range data.Resources {
				if strings.Contains(m.outPath, r.PluralLower) || strings.Contains(m.outPath, r.NameLower) {
					td.Resource = r
					break
				}
			}
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
	data, err := os.ReadFile(specPath)
	if err != nil {
		return "/api/v1"
	}
	var doc struct {
		Paths map[string]interface{} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "/api/v1"
	}
	for pathKey := range doc.Paths {
		if strings.HasPrefix(pathKey, "/api/") {
			parts := strings.Split(pathKey, "/")
			if len(parts) >= 4 {
				return "/" + parts[1] + "/" + parts[2] + "/" + parts[3]
			}
		}
	}
	return "/api/v1"
}

func inferPathSegmentFromPaths(paths map[string]interface{}, apiPrefix, resourceName string) string {
	for pathKey := range paths {
		if !strings.HasPrefix(pathKey, apiPrefix+"/") {
			continue
		}
		remainder := strings.TrimPrefix(pathKey, apiPrefix+"/")
		segments := strings.Split(remainder, "/")
		if len(segments) >= 1 {
			segment := segments[0]
			segmentPascal := toPascalCase(segment)
			if segmentPascal == resourceName || segmentPascal == resourceName+"s" {
				return segment
			}
		}
	}
	return toSnakeCase(resourceName) + "s"
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
