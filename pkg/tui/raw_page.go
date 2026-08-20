package tui

import (
	"encoding/json"
	"fmt"
	"strings"
)

type rawJSONTokenKind uint8

const (
	rawJSONWhitespace rawJSONTokenKind = iota
	rawJSONKey
	rawJSONString
	rawJSONNumber
	rawJSONLiteral
	rawJSONPunctuation
)

type rawJSONToken struct {
	kind rawJSONTokenKind
	text string
}

func renderRaw(value any) (string, error) {
	content, err := json.MarshalIndent(sanitizeRawValue(value), "", "  ")
	if err != nil {
		return "", fmt.Errorf("format raw resource: %w", err)
	}
	return string(content), nil
}

func renderRawCode(value string, theme Theme) string {
	var result strings.Builder
	result.Grow(len(value))
	for _, token := range tokenizeRawJSON(value) {
		switch token.kind {
		case rawJSONKey:
			result.WriteString(theme.RawKey(token.text))
		case rawJSONString:
			result.WriteString(theme.RawString(token.text))
		case rawJSONNumber:
			result.WriteString(theme.RawNumber(token.text))
		case rawJSONLiteral:
			result.WriteString(theme.RawLiteral(token.text))
		case rawJSONPunctuation:
			result.WriteString(theme.RawPunctuation(token.text))
		default:
			result.WriteString(token.text)
		}
	}
	return result.String()
}

func tokenizeRawJSON(value string) []rawJSONToken {
	tokens := make([]rawJSONToken, 0, len(value)/4)
	for index := 0; index < len(value); {
		start := index
		switch value[index] {
		case ' ', '\t', '\r', '\n':
			for index < len(value) && isJSONWhitespace(value[index]) {
				index++
			}
			tokens = append(tokens, rawJSONToken{kind: rawJSONWhitespace, text: value[start:index]})
		case '"':
			index++
			for index < len(value) {
				if value[index] == '\\' {
					index = min(len(value), index+2)
					continue
				}
				index++
				if value[index-1] == '"' {
					break
				}
			}
			kind := rawJSONString
			lookahead := index
			for lookahead < len(value) && isJSONWhitespace(value[lookahead]) {
				lookahead++
			}
			if lookahead < len(value) && value[lookahead] == ':' {
				kind = rawJSONKey
			}
			tokens = append(tokens, rawJSONToken{kind: kind, text: value[start:index]})
		case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			for index < len(value) && isJSONNumberByte(value[index]) {
				index++
			}
			tokens = append(tokens, rawJSONToken{kind: rawJSONNumber, text: value[start:index]})
		default:
			kind := rawJSONPunctuation
			for _, literal := range []string{"true", "false", "null"} {
				if strings.HasPrefix(value[index:], literal) {
					index += len(literal)
					kind = rawJSONLiteral
					break
				}
			}
			if index == start {
				index++
			}
			tokens = append(tokens, rawJSONToken{kind: kind, text: value[start:index]})
		}
	}
	return tokens
}

func isJSONWhitespace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
}

func isJSONNumberByte(value byte) bool {
	return value == '-' || value == '+' || value == '.' || value == 'e' || value == 'E' || (value >= '0' && value <= '9')
}

func sanitizeRawValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for _, key := range sortedAnyKeys(typed) {
			safeKey := SanitizeCell(key)
			if safeKey == "" {
				safeKey = "field"
			}
			base := safeKey
			for suffix := 2; ; suffix++ {
				if _, exists := result[safeKey]; !exists {
					break
				}
				safeKey = fmt.Sprintf("%s [%d]", base, suffix)
			}
			result[safeKey] = sanitizeRawValue(typed[key])
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index := range typed {
			result[index] = sanitizeRawValue(typed[index])
		}
		return result
	case string:
		return Sanitize(typed)
	default:
		return typed
	}
}
