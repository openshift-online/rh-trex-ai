package generator

import (
	"fmt"
	"strings"
)

// Field describes a single custom field on a generated entity.
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

	var baseType string
	var pointerType string

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
		return field, fmt.Errorf("unsupported field type: %s (supported types: string, int, int64, bool, float, time)", fieldType)
	}

	if nullable {
		field.GoType = pointerType
	} else {
		field.GoType = baseType
	}
	field.PointerType = pointerType

	return field, nil
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

func pluralize(word, explicitPlural string) string {
	if explicitPlural != "" {
		return explicitPlural
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

func article(s string) string {
	if s == "" {
		return "a"
	}
	switch s[0] {
	case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':
		return "an"
	default:
		return "a"
	}
}
