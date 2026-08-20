package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const detailColumnGap = 2

type detailField struct {
	key   string
	value string
}

func (component *DetailStreamComponent) ShowDetail(value any, theme Theme) {
	component.detailValue = value
	component.detail.SetContent(renderDetailBody(value, theme, component.detail.Width))
	component.detail.GotoTop()
}

func (component *DetailStreamComponent) ShowRaw(raw string, theme Theme) {
	component.detailValue = nil
	component.detail.SetContent(renderRawCode(raw, theme))
	component.detail.GotoTop()
}

func (component *DetailStreamComponent) Resize(width, height int, theme Theme) {
	component.detail.Width = max(1, width)
	component.detail.Height = max(1, height)
	if component.detailValue != nil {
		component.detail.SetContent(renderDetailBody(component.detailValue, theme, component.detail.Width))
	}
}

func renderDetail(value any) string {
	fields := detailFields(value)
	lines := make([]string, 0, len(fields))
	for _, field := range fields {
		lines = append(lines, field.key+": "+field.value)
	}
	return strings.Join(lines, "\n")
}

func renderDetailBody(value any, theme Theme, width int) string {
	fields := detailFields(value)
	if len(fields) == 0 || width <= 0 {
		return ""
	}

	keyWidth := 0
	for _, field := range fields {
		keyWidth = max(keyWidth, ansi.StringWidth(field.key))
	}
	keyWidth = min(keyWidth, max(1, width/3))
	gap := detailColumnGap
	if width-keyWidth <= gap {
		gap = max(0, width-keyWidth-1)
	}
	valueWidth := max(1, width-keyWidth-gap)
	prefixWidth := keyWidth + gap

	lines := make([]string, 0, len(fields))
	for _, field := range fields {
		key := ansi.Truncate(field.key, keyWidth, "…")
		keyPadding := strings.Repeat(" ", max(0, keyWidth-ansi.StringWidth(key)))
		wrapped := strings.Split(ansi.Wrap(field.value, valueWidth, " "), "\n")
		if len(wrapped) == 0 {
			wrapped = []string{""}
		}
		lines = append(lines, keyPadding+theme.DetailKeyText(key)+strings.Repeat(" ", gap)+theme.DetailValueText(wrapped[0]))
		for _, continuation := range wrapped[1:] {
			lines = append(lines, strings.Repeat(" ", prefixWidth)+theme.DetailValueText(continuation))
		}
	}
	return strings.Join(lines, "\n")
}

func detailFields(value any) []detailField {
	var fields []detailField
	flattenDetailFields("", value, 0, &fields)
	return fields
}

func flattenDetailFields(prefix string, value any, depth int, fields *[]detailField) {
	if depth > 8 {
		*fields = append(*fields, detailField{key: SanitizeCell(prefix), value: "…"})
		return
	}
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range sortedAnyKeys(typed) {
			name := key
			if prefix != "" {
				name = prefix + "." + key
			}
			flattenDetailFields(name, typed[key], depth+1, fields)
		}
	case []any:
		for index, item := range typed {
			flattenDetailFields(prefix+"["+scalarString(index)+"]", item, depth+1, fields)
		}
	default:
		*fields = append(*fields, detailField{key: SanitizeCell(prefix), value: Sanitize(scalarString(typed))})
	}
}
