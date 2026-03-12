package generate

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/binary"
	"fmt"
	"strings"
	"text/template"
)

//go:embed all:templates
var embeddedTemplates embed.FS

type RenderRequest struct {
	Kind       string
	Fields     string
	Plural     string
	Generators []string
}

type RenderedFile struct {
	Path      string
	Content   string
	Generator string
}

type Field struct {
	Name               string
	Type               string
	GoType             string
	DBType             string
	OpenAPIType        string
	OpenAPIFormat      string
	NameSnakeCase      string
	NameCamelCase      string
	JSONTag            string
	GormTag            string
	Required           bool
	Nullable           bool
	PointerType        string
	NeedsIntConversion bool
}

type templateData struct {
	Repo                string
	Project             string
	ProjectPascalCase   string
	ApiProject          string
	Library             string
	Cmd                 string
	Kind                string
	KindPlural          string
	KindLowerPlural     string
	KindLowerSingular   string
	KindSnakeCasePlural string
	ID                  string
	Fields              []Field
}

var irregularPlurals = map[string]string{
	"registry":   "registries",
	"category":   "categories",
	"company":    "companies",
	"country":    "countries",
	"family":     "families",
	"policy":     "policies",
	"directory":  "directories",
	"repository": "repositories",
	"security":   "securities",
}

type Renderer struct {
	repo       string
	project    string
	apiProject string
	library    string
	cmd        string
}

func NewRenderer() *Renderer {
	return &Renderer{
		repo:       "github.com/openshift-online",
		project:    "rh-trex-ai",
		apiProject: "rh-trex-ai",
		library:    "github.com/openshift-online/rh-trex-ai",
		cmd:        "trex",
	}
}

type templateDef struct {
	name      string
	file      string
	generator string
}

var entityTemplates = []templateDef{
	{"api", "generate-api.txt", "entity"},
	{"presenters", "generate-presenters.txt", "entity"},
	{"dao", "generate-dao.txt", "entity"},
	{"services", "generate-services.txt", "entity"},
	{"mock", "generate-mock.txt", "entity"},
	{"migration", "generate-migration.txt", "entity"},
	{"handlers", "generate-handlers.txt", "entity"},
	{"test", "generate-test.txt", "entity"},
	{"test-factories", "generate-test-factories.txt", "entity"},
	{"testmain", "generate-testmain.txt", "entity"},
	{"openapi-kind", "generate-openapi-kind.txt", "entity"},
	{"grpc-handler", "generate-grpc-handler.txt", "entity"},
	{"grpc-presenter", "generate-grpc-presenter.txt", "entity"},
	{"grpc-test", "generate-grpc-test.txt", "entity"},
	{"proto", "generate-proto.txt", "entity"},
	{"plugin", "generate-plugin.txt", "entity"},
}

func (r *Renderer) Render(req RenderRequest) ([]RenderedFile, error) {
	parsedFields, err := parseFields(req.Fields)
	if err != nil {
		return nil, fmt.Errorf("field parsing failed: %w", err)
	}

	kindLowerCamel := strings.ToLower(string(req.Kind[0])) + req.Kind[1:]
	kindPlural := pluralizeWord(req.Kind, req.Plural)
	kindPluralLower := pluralizeWord(kindLowerCamel, req.Plural)
	kindPluralSnake := toSnakeCase(kindPlural)

	migrationID := stableMigrationID(req.Kind)

	data := templateData{
		Repo:                r.repo,
		Project:             r.project,
		ProjectPascalCase:   toPascalCase(r.project),
		ApiProject:          r.apiProject,
		Library:             r.library,
		Cmd:                 r.cmd,
		Kind:                req.Kind,
		KindPlural:          kindPlural,
		KindLowerPlural:     kindPluralLower,
		KindLowerSingular:   kindLowerCamel,
		KindSnakeCasePlural: kindPluralSnake,
		ID:                  migrationID,
		Fields:              parsedFields,
	}

	generators := req.Generators
	if len(generators) == 0 {
		generators = []string{"entity"}
	}

	generatorSet := make(map[string]bool, len(generators))
	for _, g := range generators {
		generatorSet[g] = true
	}

	var allTemplates []templateDef
	if generatorSet["entity"] {
		allTemplates = append(allTemplates, entityTemplates...)
	}

	var results []RenderedFile
	funcMap := template.FuncMap{
		"protoFieldType": protoFieldType,
		"add":            func(a, b int) int { return a + b },
	}

	for _, td := range allTemplates {
		content, err := embeddedTemplates.ReadFile("templates/" + td.file)
		if err != nil {
			return nil, fmt.Errorf("reading template %s: %w", td.file, err)
		}

		tmpl, err := template.New(td.name).Funcs(funcMap).Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("parsing template %s: %w", td.name, err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, fmt.Errorf("executing template %s: %w", td.name, err)
		}

		outPath := outputPath(td.name, data)
		results = append(results, RenderedFile{
			Path:      outPath,
			Content:   buf.String(),
			Generator: td.generator,
		})
	}

	return results, nil
}

func outputPath(templateName string, data templateData) string {
	paths := map[string]string{
		"api":            fmt.Sprintf("plugins/%s/model.go", data.KindLowerPlural),
		"presenters":     fmt.Sprintf("plugins/%s/presenter.go", data.KindLowerPlural),
		"dao":            fmt.Sprintf("plugins/%s/dao.go", data.KindLowerPlural),
		"handlers":       fmt.Sprintf("plugins/%s/handler.go", data.KindLowerPlural),
		"migration":      fmt.Sprintf("plugins/%s/migration.go", data.KindLowerPlural),
		"mock":           fmt.Sprintf("plugins/%s/mock_dao.go", data.KindLowerPlural),
		"openapi-kind":   fmt.Sprintf("openapi/openapi.%s.yaml", data.KindLowerPlural),
		"grpc-handler":   fmt.Sprintf("plugins/%s/grpc_handler.go", data.KindLowerPlural),
		"grpc-presenter": fmt.Sprintf("plugins/%s/grpc_presenter.go", data.KindLowerPlural),
		"grpc-test":      fmt.Sprintf("plugins/%s/grpc_integration_test.go", data.KindLowerPlural),
		"proto":          fmt.Sprintf("proto/rh_trex/v1/%s.proto", data.KindSnakeCasePlural),
		"test-factories": fmt.Sprintf("plugins/%s/factory_test.go", data.KindLowerPlural),
		"test":           fmt.Sprintf("plugins/%s/integration_test.go", data.KindLowerPlural),
		"testmain":       fmt.Sprintf("plugins/%s/testmain_test.go", data.KindLowerPlural),
		"services":       fmt.Sprintf("plugins/%s/service.go", data.KindLowerPlural),
		"plugin":         fmt.Sprintf("plugins/%s/plugin.go", data.KindLowerPlural),
	}
	if p, ok := paths[templateName]; ok {
		return p
	}
	return templateName
}

func parseFields(fieldsStr string) ([]Field, error) {
	if fieldsStr == "" {
		return []Field{}, nil
	}

	var fields []Field
	fieldPairs := strings.Split(fieldsStr, ",")
	for _, pair := range fieldPairs {
		parts := strings.Split(strings.TrimSpace(pair), ":")
		if len(parts) < 2 || len(parts) > 3 {
			return nil, fmt.Errorf("invalid field format: %s (expected name:type or name:type:required)", pair)
		}

		name := strings.TrimSpace(parts[0])
		fieldType := strings.TrimSpace(parts[1])
		nullable := true

		if len(parts) == 3 {
			modifier := strings.TrimSpace(parts[2])
			switch modifier {
			case "required":
				nullable = false
			case "optional":
				nullable = true
			default:
				return nil, fmt.Errorf("invalid field modifier: %s (expected 'required' or 'optional')", modifier)
			}
		}

		field, err := mapFieldType(name, fieldType, nullable)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func mapFieldType(name, fieldType string, nullable bool) (Field, error) {
	goName := toPascalCase(name)
	snakeName := toSnakeCase(goName)
	camelName := toCamelCase(goName)

	field := Field{
		Name:          goName,
		Type:          fieldType,
		NameSnakeCase: snakeName,
		NameCamelCase: camelName,
		JSONTag:       fmt.Sprintf("`json:\"%s\"`", snakeName),
		Required:      !nullable,
		Nullable:      nullable,
	}

	var baseType, pointerType string

	switch fieldType {
	case "string":
		baseType = "string"
		pointerType = "*string"
		field.DBType = "text"
		field.OpenAPIType = "string"
	case "int":
		baseType = "int"
		pointerType = "*int"
		field.DBType = "integer"
		field.OpenAPIType = "integer"
		field.OpenAPIFormat = "int32"
		field.NeedsIntConversion = true
	case "int64":
		baseType = "int64"
		pointerType = "*int64"
		field.DBType = "bigint"
		field.OpenAPIType = "integer"
		field.OpenAPIFormat = "int64"
	case "bool":
		baseType = "bool"
		pointerType = "*bool"
		field.DBType = "boolean"
		field.OpenAPIType = "boolean"
	case "float":
		baseType = "float64"
		pointerType = "*float64"
		field.DBType = "double precision"
		field.OpenAPIType = "number"
		field.OpenAPIFormat = "double"
	case "time":
		baseType = "time.Time"
		pointerType = "*time.Time"
		field.DBType = "timestamp"
		field.OpenAPIType = "string"
		field.OpenAPIFormat = "date-time"
	default:
		return field, fmt.Errorf("unsupported field type: %s (supported: string, int, int64, bool, float, time)", fieldType)
	}

	if nullable {
		field.GoType = pointerType
	} else {
		field.GoType = baseType
	}
	field.PointerType = pointerType

	return field, nil
}

func pluralizeWord(word, override string) string {
	if override != "" {
		return override
	}

	wordLower := strings.ToLower(word)
	for singular, irregularPlural := range irregularPlurals {
		if strings.HasSuffix(wordLower, singular) {
			prefix := word[:len(word)-len(singular)]
			suffix := irregularPlural
			originalSuffix := word[len(word)-len(singular):]
			if originalSuffix == strings.ToUpper(originalSuffix) {
				suffix = strings.ToUpper(irregularPlural)
			} else if len(originalSuffix) > 0 && originalSuffix[0] == strings.ToUpper(string(originalSuffix[0]))[0] {
				suffix = strings.ToUpper(string(irregularPlural[0])) + irregularPlural[1:]
			}
			return prefix + suffix
		}
	}
	return word + "s"
}

func stableMigrationID(kind string) string {
	h := sha256.Sum256([]byte(kind))
	n := binary.BigEndian.Uint64(h[:8])
	return fmt.Sprintf("%012d", n%1000000000000)
}

func kindNameHash(kind string) int {
	h := sha256.Sum256([]byte(kind))
	return int(binary.BigEndian.Uint16(h[:2])) % 10000
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

func toPascalCase(s string) string {
	if len(s) == 0 {
		return s
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-'
	})
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(string(part[0])) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

func toCamelCase(s string) string {
	pascal := toPascalCase(s)
	if len(pascal) == 0 {
		return pascal
	}
	return strings.ToLower(string(pascal[0])) + pascal[1:]
}

func protoFieldType(field Field) string {
	switch field.Type {
	case "string":
		return "string"
	case "int":
		return "int32"
	case "int64":
		return "int64"
	case "bool":
		return "bool"
	case "float":
		return "double"
	case "time":
		return "google.protobuf.Timestamp"
	default:
		return "string"
	}
}
