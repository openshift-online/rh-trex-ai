package generate

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
)

type consoleResource struct {
	Name           string
	NameLower      string
	NameKebab      string
	Plural         string
	PluralLower    string
	PluralKebab    string
	PathSegment    string
	Columns        []consoleColumn
	WritableFields []consoleField
	HasDelete      bool
	HasPatch       bool
}

type consoleColumn struct {
	Name      string
	Header    string
	JSONPath  string
	FieldType string
	Sortable  bool
}

type consoleField struct {
	Name        string
	Label       string
	JSONName    string
	FieldType   string
	TSType      string
	Required    bool
	Placeholder string
}

type consoleData struct {
	PluginName  string
	DisplayName string
	Project     string
	APIPrefix   string
	NavSection  string
	Perspective string
	Resources   []consoleResource
	Resource    consoleResource
}

func buildConsoleResources(entities []ERDEntity) []consoleResource {
	var resources []consoleResource
	for _, e := range entities {
		pathSegment := toSnakeCase(e.Kind) + "s"
		nameLower := sdkLowerFirst(e.Kind)
		nameKebab := strings.ReplaceAll(toSnakeCase(e.Kind), "_", "-")
		pluralName := sdkPluralize(e.Kind)
		pluralLower := sdkLowerFirst(pluralName)
		pluralKebab := strings.ReplaceAll(toSnakeCase(pluralName), "_", "-")

		columns := buildConsoleColumns(e.Fields)
		fields := buildConsoleFields(e.Fields)

		resources = append(resources, consoleResource{
			Name:           e.Kind,
			NameLower:      nameLower,
			NameKebab:      nameKebab,
			Plural:         pluralName,
			PluralLower:    pluralLower,
			PluralKebab:    pluralKebab,
			PathSegment:    pathSegment,
			Columns:        columns,
			WritableFields: fields,
			HasDelete:      true,
			HasPatch:       true,
		})
	}
	return resources
}

func buildConsoleColumns(fieldsStr string) []consoleColumn {
	cols := []consoleColumn{
		{Name: "id", Header: "ID", JSONPath: "id", FieldType: "string", Sortable: true},
	}
	if fieldsStr != "" {
		count := 0
		for _, pair := range strings.Split(fieldsStr, ",") {
			parts := strings.Split(strings.TrimSpace(pair), ":")
			if len(parts) < 2 {
				continue
			}
			name := parts[0]
			fieldType := parts[1]
			oaType, oaFormat := sdkOpenAPIType(fieldType)
			displayType := oaType
			if oaFormat == "date-time" {
				displayType = "date-time"
			}
			cols = append(cols, consoleColumn{
				Name:      name,
				Header:    consoleColumnHeader(name),
				JSONPath:  name,
				FieldType: displayType,
				Sortable:  true,
			})
			count++
			if count >= 4 {
				break
			}
		}
	}
	cols = append(cols, consoleColumn{Name: "created_at", Header: "Created", JSONPath: "created_at", FieldType: "date-time", Sortable: true})
	return cols
}

func buildConsoleFields(fieldsStr string) []consoleField {
	if fieldsStr == "" {
		return nil
	}
	var fields []consoleField
	for _, pair := range strings.Split(fieldsStr, ",") {
		parts := strings.Split(strings.TrimSpace(pair), ":")
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		fieldType := parts[1]
		required := len(parts) == 3 && parts[2] == "required"

		tsType := "string"
		placeholder := "Enter " + consoleColumnHeader(name)
		switch fieldType {
		case "int", "int64":
			tsType = "number"
			placeholder = "0"
		case "float":
			tsType = "number"
			placeholder = "0.0"
		case "bool":
			tsType = "boolean"
			placeholder = ""
		case "time":
			placeholder = "YYYY-MM-DDTHH:MM:SSZ"
		}

		fields = append(fields, consoleField{
			Name:        sdkCamelCase(name),
			Label:       consoleColumnHeader(name),
			JSONName:    name,
			FieldType:   fieldType,
			TSType:      tsType,
			Required:    required,
			Placeholder: placeholder,
		})
	}
	return fields
}

func consoleColumnHeader(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' })
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(string(p[0])) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

var consoleFuncMap = template.FuncMap{
	"lower":     strings.ToLower,
	"upper":     strings.ToUpper,
	"title":     consoleColumnHeader,
	"kebabCase": func(s string) string { return strings.ReplaceAll(toSnakeCase(s), "_", "-") },
	"camelCase": sdkCamelCase,
	"sub":       func(a, b int) int { return a - b },
	"add":       func(a, b int) int { return a + b },
	"patternFlyInputType": func(fieldType string) string {
		switch fieldType {
		case "int", "int64", "float":
			return "number"
		default:
			return "text"
		}
	},
}

type consoleTmplDef struct {
	tmplFile string
	outPath  string
	perRes   bool
}

var consoleStaticTemplates = []consoleTmplDef{
	{"console/package.json.tmpl", "console/package.json", false},
	{"console/tsconfig.json.tmpl", "console/tsconfig.json", false},
	{"console/webpack.config.ts.tmpl", "console/webpack.config.ts", false},
	{"console/console-extensions.json.tmpl", "console/console-extensions.json", false},
	{"console/Dockerfile.tmpl", "console/Dockerfile", false},
	{"console-src/index.ts.tmpl", "console/src/index.ts", false},
	{"console-src-utils/api.ts.tmpl", "console/src/utils/api.ts", false},
	{"console-src-components/App.tsx.tmpl", "console/src/components/App.tsx", false},
	{"console-src-components/ResourceNav.tsx.tmpl", "console/src/components/ResourceNav.tsx", false},
	{"console-src-hooks/useApiAuth.ts.tmpl", "console/src/hooks/useApiAuth.ts", false},
	{"console-deploy/consoleplugin.yaml.tmpl", "console/deploy/consoleplugin.yaml", false},
	{"console-deploy/deployment.yaml.tmpl", "console/deploy/deployment.yaml", false},
	{"console-deploy/service.yaml.tmpl", "console/deploy/service.yaml", false},
	{"console-deploy/nginx.configmap.yaml.tmpl", "console/deploy/nginx-configmap.yaml", false},
}

var consolePerResourceTemplates = []consoleTmplDef{
	{"console-src-components/ListPage.tsx.tmpl", "", true},
	{"console-src-components/DetailsPage.tsx.tmpl", "", true},
	{"console-src-components/CreatePage.tsx.tmpl", "", true},
}

func renderConsole(entities []ERDEntity, project, apiPrefix string) ([]RenderedFile, error) {
	resources := buildConsoleResources(entities)
	pluginName := project + "-console"
	displayName := consoleColumnHeader(project) + " Console"

	data := consoleData{
		PluginName:  pluginName,
		DisplayName: displayName,
		Project:     project,
		APIPrefix:   apiPrefix,
		NavSection:  "home",
		Perspective: "admin",
		Resources:   resources,
	}

	var results []RenderedFile

	for _, td := range consoleStaticTemplates {
		content, err := embeddedTemplates.ReadFile("templates/" + td.tmplFile)
		if err != nil {
			return nil, fmt.Errorf("reading console template %s: %w", td.tmplFile, err)
		}
		tmpl, err := template.New(td.tmplFile).Funcs(consoleFuncMap).Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("parsing console template %s: %w", td.tmplFile, err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, fmt.Errorf("executing console template %s: %w", td.tmplFile, err)
		}
		results = append(results, RenderedFile{
			Path:      td.outPath,
			Content:   buf.String(),
			Generator: "console",
		})
	}

	for _, td := range consolePerResourceTemplates {
		content, err := embeddedTemplates.ReadFile("templates/" + td.tmplFile)
		if err != nil {
			return nil, fmt.Errorf("reading console template %s: %w", td.tmplFile, err)
		}
		for _, r := range resources {
			tmpl, err := template.New(td.tmplFile).Funcs(consoleFuncMap).Parse(string(content))
			if err != nil {
				return nil, fmt.Errorf("parsing console template %s: %w", td.tmplFile, err)
			}
			rd := data
			rd.Resource = r
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, rd); err != nil {
				return nil, fmt.Errorf("executing console template %s for %s: %w", td.tmplFile, r.Name, err)
			}
			base := filepath.Base(td.tmplFile)
			var outPath string
			switch {
			case strings.Contains(base, "ListPage"):
				outPath = "console/src/components/" + r.Name + "ListPage.tsx"
			case strings.Contains(base, "DetailsPage"):
				outPath = "console/src/components/" + r.Name + "DetailsPage.tsx"
			case strings.Contains(base, "CreatePage"):
				outPath = "console/src/components/" + r.Name + "CreatePage.tsx"
			}
			results = append(results, RenderedFile{
				Path:      outPath,
				Content:   buf.String(),
				Generator: "console",
			})
		}
	}

	return results, nil
}
