package tui

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func BuildPath(operation Operation, values map[string]any) (string, error) {
	parameters := make(map[string]Parameter)
	for _, parameter := range operation.Parameters {
		if parameter.In == "path" {
			parameters[parameter.Name] = parameter
		}
	}
	var path strings.Builder
	for _, part := range operation.PathParts {
		if part.Parameter == "" {
			path.WriteString(part.Literal)
			continue
		}
		parameter, ok := parameters[part.Parameter]
		if !ok {
			return "", fmt.Errorf("operation %s lacks path parameter %s", operation.ID, part.Parameter)
		}
		value, ok := operationParameterValue(operation, parameter, values)
		if !ok {
			return "", fmt.Errorf("operation %s requires path parameter %s", operation.ID, part.Parameter)
		}
		serialized, err := serializePath(parameter, value)
		if err != nil {
			return "", fmt.Errorf("operation %s path parameter %s: %w", operation.ID, part.Parameter, err)
		}
		path.WriteString(serialized)
	}
	return path.String(), nil
}

func BuildQueryAndHeaders(operation Operation, values map[string]any) (string, map[string]string, error) {
	var query []queryPair
	headers := make(map[string]string)
	for _, parameter := range operation.Parameters {
		if parameter.In != "query" && parameter.In != "header" {
			continue
		}
		value, present := operationParameterValue(operation, parameter, values)
		if !present {
			if parameter.Required {
				return "", nil, fmt.Errorf("operation %s requires %s parameter %s", operation.ID, parameter.In, parameter.Name)
			}
			continue
		}
		if err := validateParameter(parameter, value); err != nil {
			return "", nil, fmt.Errorf("operation %s %s parameter %s: %w", operation.ID, parameter.In, parameter.Name, err)
		}
		if parameter.In == "header" {
			serialized, err := serializeSimple(parameter, value)
			if err != nil {
				return "", nil, err
			}
			if strings.ContainsAny(serialized, "\r\n") {
				return "", nil, fmt.Errorf("header value contains a line break")
			}
			headers[parameter.Name] = serialized
			continue
		}
		var err error
		query, err = serializeQuery(query, parameter, value)
		if err != nil {
			return "", nil, err
		}
	}
	return encodeQuery(query), headers, nil
}

// ParameterValueKey returns the collision-safe key for a parameter value in a
// RequestInput.Values map. Bare parameter names remain accepted only when the
// operation has no same-named parameter in another location; bare path values
// are also accepted for navigation bindings created by older descriptors.
func ParameterValueKey(location, name string) string {
	return location + "\x00" + name
}

func operationParameterValue(operation Operation, parameter Parameter, values map[string]any) (any, bool) {
	if value, ok := values[ParameterValueKey(parameter.In, parameter.Name)]; ok {
		return value, true
	}
	duplicates := 0
	for _, candidate := range operation.Parameters {
		if candidate.Name == parameter.Name {
			duplicates++
		}
	}
	value, ok := values[parameter.Name]
	if !ok || (duplicates > 1 && parameter.In != "path") {
		return nil, false
	}
	return value, true
}

type queryPair struct {
	name          string
	value         string
	allowReserved bool
}

func serializeQuery(result []queryPair, parameter Parameter, value any) ([]queryPair, error) {
	add := func(name, value string) {
		result = append(result, queryPair{name: name, value: value, allowReserved: parameter.AllowReserved})
	}
	switch typed := value.(type) {
	case []any:
		parts := scalarSlice(typed)
		if parameter.Style == "spaceDelimited" {
			add(parameter.Name, strings.Join(parts, " "))
		} else if parameter.Style == "pipeDelimited" {
			add(parameter.Name, strings.Join(parts, "|"))
		} else if parameter.Explode {
			for _, part := range parts {
				add(parameter.Name, part)
			}
		} else {
			add(parameter.Name, strings.Join(parts, ","))
		}
	case map[string]any:
		keys := sortedAnyKeys(typed)
		if parameter.Style == "deepObject" {
			for _, key := range keys {
				add(parameter.Name+"["+key+"]", scalarString(typed[key]))
			}
		} else if parameter.Explode {
			for _, key := range keys {
				add(key, scalarString(typed[key]))
			}
		} else {
			var parts []string
			for _, key := range keys {
				parts = append(parts, key, scalarString(typed[key]))
			}
			add(parameter.Name, strings.Join(parts, ","))
		}
	default:
		add(parameter.Name, scalarString(typed))
	}
	return result, nil
}

func encodeQuery(pairs []queryPair) string {
	var result strings.Builder
	for index, pair := range pairs {
		if index > 0 {
			result.WriteByte('&')
		}
		result.WriteString(url.QueryEscape(pair.name))
		result.WriteByte('=')
		result.WriteString(encodeQueryValue(pair.value, pair.allowReserved))
	}
	return result.String()
}

func encodeQueryValue(value string, allowReserved bool) string {
	encoded := url.QueryEscape(value)
	if !allowReserved {
		return encoded
	}
	for _, character := range ":/?#[]@!$&'()*+,;=" {
		escaped := url.QueryEscape(string(character))
		encoded = strings.ReplaceAll(encoded, escaped, string(character))
	}
	return encoded
}

func serializePath(parameter Parameter, value any) (string, error) {
	if err := validateParameter(parameter, value); err != nil {
		return "", err
	}
	style := parameter.Style
	if style == "" {
		style = "simple"
	}
	scalar := func(value any) string { return url.PathEscape(scalarString(value)) }
	switch typed := value.(type) {
	case []any:
		values := make([]string, len(typed))
		for index, item := range typed {
			values[index] = scalar(item)
		}
		switch style {
		case "simple":
			return strings.Join(values, ","), nil
		case "label":
			separator := ","
			if parameter.Explode {
				separator = "."
			}
			return "." + strings.Join(values, separator), nil
		case "matrix":
			if parameter.Explode {
				return ";" + parameter.Name + "=" + strings.Join(values, ";"+parameter.Name+"="), nil
			}
			return ";" + parameter.Name + "=" + strings.Join(values, ","), nil
		}
	case map[string]any:
		keys := sortedAnyKeys(typed)
		var pairs []string
		for _, key := range keys {
			encodedKey, encodedValue := scalar(key), scalar(typed[key])
			if parameter.Explode {
				pairs = append(pairs, encodedKey+"="+encodedValue)
			} else {
				pairs = append(pairs, encodedKey, encodedValue)
			}
		}
		switch style {
		case "simple":
			return strings.Join(pairs, ","), nil
		case "label":
			separator := ","
			if parameter.Explode {
				separator = "."
			}
			return "." + strings.Join(pairs, separator), nil
		case "matrix":
			if parameter.Explode {
				return ";" + strings.Join(pairs, ";"), nil
			}
			return ";" + parameter.Name + "=" + strings.Join(pairs, ","), nil
		}
	default:
		switch style {
		case "simple":
			return scalar(typed), nil
		case "label":
			return "." + scalar(typed), nil
		case "matrix":
			return ";" + parameter.Name + "=" + scalar(typed), nil
		}
	}
	return "", fmt.Errorf("unsupported path style %q", style)
}

func serializeSimple(parameter Parameter, value any) (string, error) {
	if err := validateParameter(parameter, value); err != nil {
		return "", err
	}
	switch typed := value.(type) {
	case []any:
		return strings.Join(scalarSlice(typed), ","), nil
	case map[string]any:
		keys := sortedAnyKeys(typed)
		var parts []string
		for _, key := range keys {
			if parameter.Explode {
				parts = append(parts, key+"="+scalarString(typed[key]))
			} else {
				parts = append(parts, key, scalarString(typed[key]))
			}
		}
		return strings.Join(parts, ","), nil
	default:
		return scalarString(typed), nil
	}
}

func validateParameter(parameter Parameter, value any) error {
	text := scalarString(value)
	switch parameter.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("must be a string")
		}
	case "integer":
		if _, err := strconv.ParseInt(text, 10, 64); err != nil {
			return fmt.Errorf("must be an integer")
		}
	case "number":
		if _, err := strconv.ParseFloat(text, 64); err != nil {
			return fmt.Errorf("must be a number")
		}
	case "boolean":
		if _, err := strconv.ParseBool(text); err != nil {
			return fmt.Errorf("must be a boolean")
		}
	case "array":
		if _, ok := value.([]any); !ok {
			return fmt.Errorf("must be an array")
		}
	case "object":
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("must be an object")
		}
	}
	if parameter.Pattern != "" {
		pattern, err := regexp.Compile(parameter.Pattern)
		if err != nil {
			return fmt.Errorf("has invalid schema pattern")
		}
		if !pattern.MatchString(text) {
			return fmt.Errorf("does not match schema pattern")
		}
	}
	return nil
}

func ResolveJSONPointer(value any, pointer string) (any, error) {
	if pointer == "" {
		return value, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("JSON pointer %q must begin with /", pointer)
	}
	current := value
	for _, raw := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		part := strings.ReplaceAll(strings.ReplaceAll(raw, "~1", "/"), "~0", "~")
		switch typed := current.(type) {
		case map[string]any:
			var ok bool
			current, ok = typed[part]
			if !ok {
				return nil, fmt.Errorf("JSON pointer %q is missing %q", pointer, part)
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, fmt.Errorf("JSON pointer %q has invalid array index %q", pointer, part)
			}
			current = typed[index]
		default:
			return nil, fmt.Errorf("JSON pointer %q traverses a scalar", pointer)
		}
	}
	return current, nil
}

func scalarString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func scalarSlice(values []any) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = scalarString(value)
	}
	return result
}

func sortedAnyKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
