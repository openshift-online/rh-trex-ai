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
	outDir := flag.String("out", "", "output directory for the console plugin project")
	pluginName := flag.String("name", "", "plugin name (DNS-compatible, e.g. rh-trex-console)")
	displayName := flag.String("display-name", "", "human-readable display name")
	projectName := flag.String("project", "", "project name (e.g. rh-trex-ai)")
	apiPrefix := flag.String("api-prefix", "", "API path prefix (e.g. /api/rh-trex-ai/v1)")
	navSection := flag.String("nav-section", "home", "OpenShift console navigation section")
	perspective := flag.String("perspective", "admin", "console perspective: admin or dev")
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

	if *pluginName == "" {
		*pluginName = *projectName + "-console"
	}

	if *displayName == "" {
		*displayName = toTitleCase(*projectName) + " Console"
	}

	resources, err := parseResources(*specPath, *apiPrefix)
	if err != nil {
		log.Fatalf("parse resources: %v", err)
	}

	fmt.Printf("Generating console plugin '%s' for %d resources\n", *pluginName, len(resources))
	for _, r := range resources {
		fmt.Printf("  %s (%s): %d columns, %d writable fields\n", r.Name, r.PathSegment, len(r.Columns), len(r.WritableFields))
	}

	data := pluginData{
		PluginName:  *pluginName,
		DisplayName: *displayName,
		Project:     *projectName,
		APIPrefix:   *apiPrefix,
		NavSection:  *navSection,
		Perspective: *perspective,
		Resources:   resources,
	}

	if err := generatePlugin(data, *outDir); err != nil {
		log.Fatalf("generate plugin: %v", err)
	}

	fmt.Printf("Console plugin generated in %s\n", *outDir)
}

type pluginResource struct {
	Name           string
	NameLower      string
	NameKebab      string
	Plural         string
	PluralLower    string
	PluralKebab    string
	PathSegment    string
	Columns        []pluginColumn
	WritableFields []pluginField
	HasDelete      bool
	HasPatch       bool
}

type pluginColumn struct {
	Name      string
	Header    string
	JSONPath  string
	FieldType string
	Sortable  bool
}

type pluginField struct {
	Name        string
	Label       string
	JSONName    string
	FieldType   string
	TSType      string
	Required    bool
	Placeholder string
}

type pluginData struct {
	PluginName  string
	DisplayName string
	Project     string
	APIPrefix   string
	NavSection  string
	Perspective string
	Resources   []pluginResource
}

type templateContext struct {
	pluginData
	Resource pluginResource
}

func parseResources(specPath, apiPrefix string) ([]pluginResource, error) {
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
	var resources []pluginResource

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
			Paths      map[string]interface{} `yaml:"paths"`
			Components struct {
				Schemas map[string]interface{} `yaml:"schemas"`
			} `yaml:"components"`
		}
		if err := yaml.Unmarshal(subData, &subDoc); err != nil {
			continue
		}

		pathSegment := inferPathSegmentFromPaths(doc.Paths, apiPrefix, schemaName)
		columns := extractColumns(subDoc.Components.Schemas, schemaName)
		fields := extractWritableFields(subDoc.Components.Schemas, schemaName)
		hasDelete := checkOperation(subDoc.Paths, pathSegment, "delete")
		hasPatch := checkOperation(subDoc.Paths, pathSegment, "patch")

		nameLower := toLowerFirst(schemaName)
		nameKebab := toKebabCase(schemaName)
		pluralName := pluralizeName(schemaName)
		pluralLower := toLowerFirst(pluralName)
		pluralKebab := toKebabCase(pluralName)

		resources = append(resources, pluginResource{
			Name:           schemaName,
			NameLower:      nameLower,
			NameKebab:      nameKebab,
			Plural:         pluralName,
			PluralLower:    pluralLower,
			PluralKebab:    pluralKebab,
			PathSegment:    pathSegment,
			Columns:        columns,
			WritableFields: fields,
			HasDelete:      hasDelete,
			HasPatch:       hasPatch,
		})
	}

	sort.Slice(resources, func(i, j int) bool {
		return resources[i].Name < resources[j].Name
	})

	return resources, nil
}

func extractColumns(schemas map[string]interface{}, name string) []pluginColumn {
	columns := []pluginColumn{
		{Name: "id", Header: "ID", JSONPath: "id", FieldType: "string", Sortable: true},
	}

	schema, ok := schemas[name]
	if !ok {
		columns = append(columns, pluginColumn{Name: "created_at", Header: "Created", JSONPath: "created_at", FieldType: "date-time", Sortable: true})
		return columns
	}

	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		columns = append(columns, pluginColumn{Name: "created_at", Header: "Created", JSONPath: "created_at", FieldType: "date-time", Sortable: true})
		return columns
	}

	var fieldNames []string
	fieldTypes := map[string]string{}

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
				for k, v := range propsMap {
					fieldNames = append(fieldNames, k)
					if propMap, ok := v.(map[string]interface{}); ok {
						t, _ := propMap["type"].(string)
						f, _ := propMap["format"].(string)
						if f != "" {
							fieldTypes[k] = f
						} else {
							fieldTypes[k] = t
						}
					}
				}
			}
		}
	}

	sort.Strings(fieldNames)

	count := 0
	for _, f := range fieldNames {
		if f == "id" || f == "kind" || f == "href" {
			continue
		}
		ft := fieldTypes[f]
		columns = append(columns, pluginColumn{
			Name:      f,
			Header:    toColumnHeader(f),
			JSONPath:  f,
			FieldType: ft,
			Sortable:  true,
		})
		count++
		if count >= 4 {
			break
		}
	}

	columns = append(columns, pluginColumn{Name: "created_at", Header: "Created", JSONPath: "created_at", FieldType: "date-time", Sortable: true})
	return columns
}

func extractWritableFields(schemas map[string]interface{}, name string) []pluginField {
	schema, ok := schemas[name]
	if !ok {
		return nil
	}

	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		return nil
	}

	var fields []pluginField
	var requiredSet map[string]bool

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
				if req, ok := itemMap["required"]; ok {
					if reqList, ok := req.([]interface{}); ok {
						requiredSet = make(map[string]bool)
						for _, r := range reqList {
							if s, ok := r.(string); ok {
								requiredSet[s] = true
							}
						}
					}
				}
				fields = append(fields, extractPropsAsFields(itemMap, requiredSet)...)
			}
		}
	} else {
		if req, ok := schemaMap["required"]; ok {
			if reqList, ok := req.([]interface{}); ok {
				requiredSet = make(map[string]bool)
				for _, r := range reqList {
					if s, ok := r.(string); ok {
						requiredSet[s] = true
					}
				}
			}
		}
		fields = extractPropsAsFields(schemaMap, requiredSet)
	}

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Name < fields[j].Name
	})

	return fields
}

func extractPropsAsFields(m map[string]interface{}, required map[string]bool) []pluginField {
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

	var fields []pluginField
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
		propFormat, _ := propMap["format"].(string)

		tsType := "string"
		placeholder := ""
		switch propType {
		case "integer":
			tsType = "number"
			placeholder = "0"
		case "number":
			tsType = "number"
			placeholder = "0.0"
		case "boolean":
			tsType = "boolean"
		default:
			placeholder = "Enter " + toColumnHeader(propName)
		}

		if propFormat == "date-time" {
			tsType = "string"
			placeholder = "YYYY-MM-DDTHH:MM:SSZ"
		}

		fields = append(fields, pluginField{
			Name:        toCamelCase(propName),
			Label:       toColumnHeader(propName),
			JSONName:    propName,
			FieldType:   propType,
			TSType:      tsType,
			Required:    required[propName],
			Placeholder: placeholder,
		})
	}
	return fields
}

func generatePlugin(data pluginData, outDir string) error {
	tmplDir := getTemplateDir()

	type tmplMapping struct {
		tmplPath    string
		outPath     string
		perResource bool
	}

	var mappings []tmplMapping

	mappings = append(mappings,
		tmplMapping{"package.json.tmpl", "package.json", false},
		tmplMapping{"tsconfig.json.tmpl", "tsconfig.json", false},
		tmplMapping{"webpack.config.ts.tmpl", "webpack.config.ts", false},
		tmplMapping{"console-extensions.json.tmpl", "console-extensions.json", false},
		tmplMapping{"Dockerfile.tmpl", "Dockerfile", false},
		tmplMapping{"src/index.ts.tmpl", "src/index.ts", false},
		tmplMapping{"src/utils/api.ts.tmpl", "src/utils/api.ts", false},
		tmplMapping{"src/components/App.tsx.tmpl", "src/components/App.tsx", false},
		tmplMapping{"src/components/ResourceNav.tsx.tmpl", "src/components/ResourceNav.tsx", false},
		tmplMapping{"deploy/consoleplugin.yaml.tmpl", filepath.Join("deploy", "consoleplugin.yaml"), false},
		tmplMapping{"deploy/deployment.yaml.tmpl", filepath.Join("deploy", "deployment.yaml"), false},
		tmplMapping{"deploy/service.yaml.tmpl", filepath.Join("deploy", "service.yaml"), false},
		tmplMapping{"deploy/nginx.configmap.yaml.tmpl", filepath.Join("deploy", "nginx-configmap.yaml"), false},
	)

	for _, r := range data.Resources {
		mappings = append(mappings,
			tmplMapping{"src/components/ListPage.tsx.tmpl", filepath.Join("src", "components", r.Name+"ListPage.tsx"), true},
			tmplMapping{"src/components/DetailsPage.tsx.tmpl", filepath.Join("src", "components", r.Name+"DetailsPage.tsx"), true},
			tmplMapping{"src/components/CreatePage.tsx.tmpl", filepath.Join("src", "components", r.Name+"CreatePage.tsx"), true},
		)
	}

	funcMap := buildFuncMap()

	for _, m := range mappings {
		tmplPath := filepath.Join(tmplDir, m.tmplPath)
		outPath := filepath.Join(outDir, m.outPath)

		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(outPath), err)
		}

		tmplData, err := os.ReadFile(tmplPath)
		if err != nil {
			return fmt.Errorf("read template %s: %w", m.tmplPath, err)
		}

		tmpl, err := template.New(filepath.Base(m.tmplPath)).Funcs(funcMap).Parse(string(tmplData))
		if err != nil {
			return fmt.Errorf("parse template %s: %w", m.tmplPath, err)
		}

		ctx := templateContext{pluginData: data}

		if m.perResource {
			for _, r := range data.Resources {
				if strings.Contains(outPath, r.Name) {
					ctx.Resource = r
					break
				}
			}
		}

		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("create %s: %w", outPath, err)
		}
		if err := tmpl.Execute(f, ctx); err != nil {
			f.Close()
			return fmt.Errorf("execute template %s: %w", m.tmplPath, err)
		}
		f.Close()
	}

	return nil
}

func buildFuncMap() template.FuncMap {
	return template.FuncMap{
		"lower":     strings.ToLower,
		"upper":     strings.ToUpper,
		"title":     toTitleCase,
		"kebabCase": toKebabCase,
		"camelCase": toCamelCase,
		"sub":       func(a, b int) int { return a - b },
		"add":       func(a, b int) int { return a + b },
		"patternFlyInputType": func(fieldType string) string {
			switch fieldType {
			case "integer", "number":
				return "number"
			default:
				return "text"
			}
		},
	}
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

func checkOperation(paths map[string]interface{}, pathSegment, operation string) bool {
	for pathKey, pathVal := range paths {
		if !strings.Contains(pathKey, "/"+pathSegment+"/{id}") {
			continue
		}
		segments := strings.Split(pathKey, "/")
		if len(segments) > 0 && segments[len(segments)-1] != "{id}" {
			continue
		}
		pathMap, ok := pathVal.(map[string]interface{})
		if !ok {
			continue
		}
		if _, has := pathMap[operation]; has {
			return true
		}
	}
	return false
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

func toTitleCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' })
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(string(p[0])) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func toColumnHeader(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' })
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(string(p[0])) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
