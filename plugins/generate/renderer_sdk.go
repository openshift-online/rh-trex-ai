package generate

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"unicode"
)

type sdkResource struct {
	Name              string
	Plural            string
	PathSegment       string
	Fields            []sdkField
	RequiredFields    []string
	PatchFields       []sdkField
	StatusPatchFields []sdkField
	HasDelete         bool
	HasPatch          bool
	HasStatusPatch    bool
	Actions           []string
}

type sdkField struct {
	Name       string
	GoName     string
	PythonName string
	TSName     string
	Type       string
	Format     string
	GoType     string
	PythonType string
	TSType     string
	Required   bool
	ReadOnly   bool
	JSONTag    string
}

type sdkSpec struct {
	Resources []sdkResource
	APIPrefix string
	Module    string
	Project   string
}

type sdkGeneratedHeader struct {
	SpecPath  string
	SpecHash  string
	Timestamp string
}

type sdkTemplateData struct {
	Header   sdkGeneratedHeader
	Resource sdkResource
	Spec     *sdkSpec
}

func buildSDKResources(entities []ERDEntity) []sdkResource {
	var resources []sdkResource
	for _, e := range entities {
		fields := erdFieldsToSDKFields(e.Fields)
		pathSegment := toSnakeCase(e.Kind) + "s"
		plural := sdkPluralize(e.Kind)
		var patchFields []sdkField
		for _, f := range fields {
			patchFields = append(patchFields, f)
		}
		var required []string
		for _, f := range fields {
			if f.Required {
				required = append(required, f.Name)
			}
		}
		resources = append(resources, sdkResource{
			Name:           e.Kind,
			Plural:         plural,
			PathSegment:    pathSegment,
			Fields:         fields,
			RequiredFields: required,
			PatchFields:    patchFields,
			HasDelete:      true,
			HasPatch:       true,
		})
	}
	return resources
}

func erdFieldsToSDKFields(fieldsStr string) []sdkField {
	if fieldsStr == "" {
		return nil
	}
	var fields []sdkField
	for _, pair := range strings.Split(fieldsStr, ",") {
		parts := strings.Split(strings.TrimSpace(pair), ":")
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		fieldType := parts[1]
		required := len(parts) == 3 && parts[2] == "required"

		goName := sdkGoName(name)
		pythonName := name
		tsName := sdkCamelCase(name)

		oaType, oaFormat := sdkOpenAPIType(fieldType)
		goType := sdkGoType(oaType, oaFormat)
		pyType := sdkPythonType(oaType, oaFormat)
		tsType := sdkTSType(oaType)

		tag := fmt.Sprintf("`json:\"%s,omitempty\"`", name)
		if required {
			tag = fmt.Sprintf("`json:\"%s\"`", name)
		}

		fields = append(fields, sdkField{
			Name:       name,
			GoName:     goName,
			PythonName: pythonName,
			TSName:     tsName,
			Type:       oaType,
			Format:     oaFormat,
			GoType:     goType,
			PythonType: pyType,
			TSType:     tsType,
			Required:   required,
			JSONTag:    tag,
		})
	}
	return fields
}

func sdkOpenAPIType(trexType string) (string, string) {
	switch trexType {
	case "string":
		return "string", ""
	case "int":
		return "integer", "int32"
	case "int64":
		return "integer", "int64"
	case "bool":
		return "boolean", ""
	case "float":
		return "number", ""
	case "time":
		return "string", "date-time"
	default:
		return "string", ""
	}
}

func sdkGoType(oaType, format string) string {
	switch oaType {
	case "string":
		if format == "date-time" {
			return "*time.Time"
		}
		return "string"
	case "integer":
		if format == "int64" {
			return "int64"
		}
		return "int"
	case "number":
		return "float64"
	case "boolean":
		return "bool"
	default:
		return "string"
	}
}

func sdkPythonType(oaType, format string) string {
	switch oaType {
	case "string":
		if format == "date-time" {
			return "Optional[datetime]"
		}
		return "str"
	case "integer":
		return "int"
	case "number":
		return "float"
	case "boolean":
		return "bool"
	default:
		return "str"
	}
}

func sdkTSType(oaType string) string {
	switch oaType {
	case "integer", "number":
		return "number"
	case "boolean":
		return "boolean"
	default:
		return "string"
	}
}

func sdkGoName(snakeName string) string {
	parts := strings.Split(snakeName, "_")
	var result strings.Builder
	acronyms := map[string]string{
		"ID": "ID", "URL": "URL", "HTTP": "HTTP", "API": "API",
		"UI": "UI", "IP": "IP", "DNS": "DNS", "TLS": "TLS",
		"UUID": "UUID", "LLM": "LLM",
	}
	for _, part := range parts {
		if part == "" {
			continue
		}
		if upper, ok := acronyms[strings.ToUpper(part)]; ok {
			result.WriteString(upper)
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		result.WriteString(string(runes))
	}
	return result.String()
}

func sdkCamelCase(snakeName string) string {
	parts := strings.Split(snakeName, "_")
	if len(parts) == 0 {
		return snakeName
	}
	var result strings.Builder
	result.WriteString(parts[0])
	for _, part := range parts[1:] {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		result.WriteString(string(runes))
	}
	return result.String()
}

func sdkPluralize(name string) string {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, "s") {
		return name + "es"
	}
	if strings.HasSuffix(lower, "y") && len(lower) > 1 {
		prev := lower[len(lower)-2]
		if prev != 'a' && prev != 'e' && prev != 'i' && prev != 'o' && prev != 'u' {
			return name[:len(name)-1] + "ies"
		}
	}
	return name + "s"
}

func sdkLowerFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

var sdkFuncMap = template.FuncMap{
	"snakeCase": toSnakeCase,
	"lower":     strings.ToLower,
	"title": func(s string) string {
		if s == "" {
			return s
		}
		r := []rune(s)
		r[0] = unicode.ToUpper(r[0])
		return string(r)
	},
	"goName": sdkGoName,
	"pythonDefault": func(oaType, format string) string {
		switch oaType {
		case "string":
			if format == "date-time" {
				return "None"
			}
			return "\"\""
		case "integer":
			return "0"
		case "number":
			return "0.0"
		case "boolean":
			return "False"
		default:
			return "\"\""
		}
	},
	"isDateTime": func(f sdkField) bool {
		return f.Format == "date-time" && strings.Contains(f.GoType, "time.Time")
	},
	"isWritable": func(f sdkField) bool { return !f.ReadOnly },
	"camelCase":  sdkCamelCase,
	"pluralize":  func(s string) string { return strings.ToLower(sdkPluralize(s)) },
	"lowerFirst": sdkLowerFirst,
	"tsDefault": func(oaType, _ string) string {
		switch oaType {
		case "integer", "number":
			return "0"
		case "boolean":
			return "false"
		default:
			return "''"
		}
	},
	"hasTimeImport": func(fields []sdkField) bool {
		for _, f := range fields {
			if f.Format == "date-time" {
				return true
			}
		}
		return false
	},
}

type sdkTemplateDef struct {
	tmplFile string
	outFunc  func(sdkResource, *sdkSpec) string
	perRes   bool
}

var goSDKTemplates = []sdkTemplateDef{
	{"sdk-go/base.go.tmpl", nil, false},
	{"sdk-go/list_options.go.tmpl", nil, false},
	{"sdk-go/iterator.go.tmpl", nil, false},
	{"sdk-go/http_client.go.tmpl", nil, false},
	{"sdk-go/types.go.tmpl", func(r sdkResource, _ *sdkSpec) string {
		return "sdk/go/types/" + toSnakeCase(r.Name) + ".go"
	}, true},
	{"sdk-go/client.go.tmpl", func(r sdkResource, _ *sdkSpec) string {
		return "sdk/go/client/" + toSnakeCase(r.Name) + "_api.go"
	}, true},
}

var pythonSDKTemplates = []sdkTemplateDef{
	{"sdk-python/__init__.py.tmpl", nil, false},
	{"sdk-python/base.py.tmpl", nil, false},
	{"sdk-python/iterator.py.tmpl", nil, false},
	{"sdk-python/http_client.py.tmpl", nil, false},
	{"sdk-python/types.py.tmpl", func(r sdkResource, _ *sdkSpec) string {
		return "sdk/python/" + toSnakeCase(r.Name) + ".py"
	}, true},
	{"sdk-python/client.py.tmpl", func(r sdkResource, _ *sdkSpec) string {
		return "sdk/python/_" + toSnakeCase(r.Name) + "_api.py"
	}, true},
}

var tsSDKTemplates = []sdkTemplateDef{
	{"sdk-ts/base.ts.tmpl", nil, false},
	{"sdk-ts/index.ts.tmpl", nil, false},
	{"sdk-ts/main_client.ts.tmpl", nil, false},
	{"sdk-ts/types.ts.tmpl", func(r sdkResource, _ *sdkSpec) string {
		return "sdk/typescript/src/" + toSnakeCase(r.Name) + ".ts"
	}, true},
	{"sdk-ts/client.ts.tmpl", func(r sdkResource, _ *sdkSpec) string {
		return "sdk/typescript/src/" + toSnakeCase(r.Name) + "_api.ts"
	}, true},
}

var goSDKStaticPaths = map[string]string{
	"sdk-go/base.go.tmpl":        "sdk/go/types/base.go",
	"sdk-go/list_options.go.tmpl": "sdk/go/types/list_options.go",
	"sdk-go/iterator.go.tmpl":    "sdk/go/client/iterator.go",
	"sdk-go/http_client.go.tmpl": "sdk/go/client/client.go",
}

var pythonSDKStaticPaths = map[string]string{
	"sdk-python/__init__.py.tmpl":    "sdk/python/__init__.py",
	"sdk-python/base.py.tmpl":        "sdk/python/_base.py",
	"sdk-python/iterator.py.tmpl":    "sdk/python/_iterator.py",
	"sdk-python/http_client.py.tmpl": "sdk/python/client.py",
}

var tsSDKStaticPaths = map[string]string{
	"sdk-ts/base.ts.tmpl":        "sdk/typescript/src/base.ts",
	"sdk-ts/index.ts.tmpl":       "sdk/typescript/src/index.ts",
	"sdk-ts/main_client.ts.tmpl": "sdk/typescript/src/client.ts",
}

func renderSDK(entities []ERDEntity, langs []string, project, apiPrefix string) ([]RenderedFile, error) {
	resources := buildSDKResources(entities)
	spec := &sdkSpec{
		Resources: resources,
		APIPrefix: apiPrefix,
		Module:    "github.com/example/" + project + "-sdk",
		Project:   project,
	}

	header := sdkGeneratedHeader{
		SpecPath:  "generated-from-erd",
		SpecHash:  "erd-generated",
		Timestamp: "deterministic",
	}

	langSet := make(map[string]bool)
	for _, l := range langs {
		langSet[l] = true
	}

	var allTemplates []sdkTemplateDef
	var staticPaths map[string]string

	if langSet["sdk-go"] || langSet["all"] {
		allTemplates = append(allTemplates, goSDKTemplates...)
		if staticPaths == nil {
			staticPaths = make(map[string]string)
		}
		for k, v := range goSDKStaticPaths {
			staticPaths[k] = v
		}
	}
	if langSet["sdk-python"] || langSet["all"] {
		allTemplates = append(allTemplates, pythonSDKTemplates...)
		if staticPaths == nil {
			staticPaths = make(map[string]string)
		}
		for k, v := range pythonSDKStaticPaths {
			staticPaths[k] = v
		}
	}
	if langSet["sdk-ts"] || langSet["all"] {
		allTemplates = append(allTemplates, tsSDKTemplates...)
		if staticPaths == nil {
			staticPaths = make(map[string]string)
		}
		for k, v := range tsSDKStaticPaths {
			staticPaths[k] = v
		}
	}

	var results []RenderedFile

	for _, td := range allTemplates {
		content, err := embeddedTemplates.ReadFile("templates/" + td.tmplFile)
		if err != nil {
			return nil, fmt.Errorf("reading sdk template %s: %w", td.tmplFile, err)
		}

		if td.perRes {
			for _, r := range resources {
				tmpl, err := template.New(td.tmplFile).Funcs(sdkFuncMap).Parse(string(content))
				if err != nil {
					return nil, fmt.Errorf("parsing sdk template %s: %w", td.tmplFile, err)
				}
				data := sdkTemplateData{Header: header, Resource: r, Spec: spec}
				var buf bytes.Buffer
				if err := tmpl.Execute(&buf, data); err != nil {
					return nil, fmt.Errorf("executing sdk template %s for %s: %w", td.tmplFile, r.Name, err)
				}
				outPath := td.outFunc(r, spec)
				results = append(results, RenderedFile{
					Path:      outPath,
					Content:   buf.String(),
					Generator: generatorFromTemplate(td.tmplFile),
				})
			}
		} else {
			tmpl, err := template.New(td.tmplFile).Funcs(sdkFuncMap).Parse(string(content))
			if err != nil {
				return nil, fmt.Errorf("parsing sdk template %s: %w", td.tmplFile, err)
			}
			data := sdkTemplateData{Header: header, Spec: spec}
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				return nil, fmt.Errorf("executing sdk template %s: %w", td.tmplFile, err)
			}
			outPath := staticPaths[td.tmplFile]
			results = append(results, RenderedFile{
				Path:      outPath,
				Content:   buf.String(),
				Generator: generatorFromTemplate(td.tmplFile),
			})
		}
	}

	return results, nil
}

func generatorFromTemplate(tmplFile string) string {
	if strings.HasPrefix(tmplFile, "sdk-go/") {
		return "sdk-go"
	}
	if strings.HasPrefix(tmplFile, "sdk-python/") {
		return "sdk-python"
	}
	if strings.HasPrefix(tmplFile, "sdk-ts/") {
		return "sdk-ts"
	}
	return "sdk"
}
