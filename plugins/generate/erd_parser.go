package generate

import (
	"fmt"
	"regexp"
	"strings"
)

type ERDEntity struct {
	Kind   string
	Fields string
}

var (
	entityHeaderPattern     = regexp.MustCompile(`^\s*(\w+)\s*\{\s*$`)
	entityFieldPattern      = regexp.MustCompile(`^\s+(\w+)\s+(\w+)(?:\s+(\w+))?(?:\s+"[^"]*")?\s*$`)
	relationshipLinePattern = regexp.MustCompile(`\|[|o{]--[|o{]\|`)
	commentPattern          = regexp.MustCompile(`^\s*%%`)
)

var mermaidTypeMap = map[string]string{
	"string":    "string",
	"varchar":   "string",
	"text":      "string",
	"uuid":      "string",
	"int":       "int",
	"integer":   "int",
	"int32":     "int",
	"int64":     "int64",
	"bigint":    "int64",
	"long":      "int64",
	"bool":      "bool",
	"boolean":   "bool",
	"float":     "float",
	"float64":   "float",
	"double":    "float",
	"decimal":   "float",
	"number":    "float",
	"date":      "time",
	"datetime":  "time",
	"timestamp": "time",
	"time":      "time",
}

func ParseERD(erd string) ([]ERDEntity, error) {
	lines := strings.Split(erd, "\n")
	var entities []ERDEntity
	var currentEntity string
	var currentFields []string
	insideBlock := false

	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r")

		if commentPattern.MatchString(line) {
			continue
		}

		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			continue
		}

		if strings.EqualFold(trimmed, "erdiagram") || strings.HasPrefix(strings.ToLower(trimmed), "erdiagram") {
			continue
		}

		if !insideBlock && relationshipLinePattern.MatchString(line) {
			continue
		}

		if !insideBlock && strings.Contains(line, "||--") {
			continue
		}

		if entityHeaderPattern.MatchString(line) {
			matches := entityHeaderPattern.FindStringSubmatch(line)
			currentEntity = normalizeEntityName(matches[1])
			currentFields = nil
			insideBlock = true
			continue
		}

		if insideBlock && trimmed == "}" {
			fieldsStr := strings.Join(currentFields, ",")
			entities = append(entities, ERDEntity{
				Kind:   currentEntity,
				Fields: fieldsStr,
			})
			insideBlock = false
			currentEntity = ""
			currentFields = nil
			continue
		}

		if insideBlock {
			matches := entityFieldPattern.FindStringSubmatch(line)
			if matches == nil {
				return nil, fmt.Errorf("unparseable field in entity %s: %q", currentEntity, trimmed)
			}
			mermaidType := strings.ToLower(matches[1])
			fieldName := matches[2]
			modifier := strings.ToUpper(matches[3])

			if fieldName == "id" && modifier == "PK" {
				continue
			}

			if fieldName == "created_at" || fieldName == "updated_at" || fieldName == "deleted_at" {
				continue
			}

			trexType, ok := mermaidTypeMap[mermaidType]
			if !ok {
				return nil, fmt.Errorf("unknown type %q for field %s.%s (supported: string, uuid, int, int64, bool, float, time, datetime, timestamp)", mermaidType, currentEntity, fieldName)
			}

			fieldSpec := fieldName + ":" + trexType
			if strings.EqualFold(modifier, "REQUIRED") || strings.EqualFold(modifier, "PK") {
				fieldSpec += ":required"
			}

			currentFields = append(currentFields, fieldSpec)
		}
	}

	if insideBlock {
		return nil, fmt.Errorf("unterminated entity block: %s (missing closing brace)", currentEntity)
	}

	if len(entities) == 0 {
		return nil, fmt.Errorf("no entities found in ERD")
	}

	return entities, nil
}

func normalizeEntityName(name string) string {
	if !strings.Contains(name, "_") && name != strings.ToUpper(name) {
		return name
	}
	parts := strings.Split(strings.ToLower(name), "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}
