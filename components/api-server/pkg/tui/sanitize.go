package tui

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var regionTagPattern = regexp.MustCompile(`\["[^"\r\n]*"\]`)

// Sanitize removes terminal control sequences and framework markup from
// untrusted strings while preserving printable Unicode, tabs, and newlines.
func Sanitize(value string) string {
	var output strings.Builder
	output.Grow(len(value))
	for index := 0; index < len(value); {
		if value[index] == 0x1b {
			index = skipEscape(value, index)
			continue
		}
		r, size := utf8.DecodeRuneInString(value[index:])
		if r == utf8.RuneError && size == 1 {
			index++
			continue
		}
		index += size
		switch {
		case r == '\n' || r == '\t':
			output.WriteRune(r)
		case r == 0x7f, r >= 0x80 && r <= 0x9f, unicode.IsControl(r):
			continue
		default:
			output.WriteRune(r)
		}
	}
	return regionTagPattern.ReplaceAllString(output.String(), "")
}

func skipEscape(value string, start int) int {
	if start+1 >= len(value) {
		return len(value)
	}
	next := value[start+1]
	switch next {
	case '[': // CSI: final byte is 0x40-0x7e.
		for index := start + 2; index < len(value); index++ {
			if value[index] >= 0x40 && value[index] <= 0x7e {
				return index + 1
			}
		}
		return len(value)
	case ']': // OSC: BEL or ST terminated.
		return skipStringEscape(value, start+2, true)
	case 'P', 'X', '^', '_': // DCS, SOS, PM, APC: ST terminated.
		return skipStringEscape(value, start+2, false)
	default:
		return start + 2
	}
}

func skipStringEscape(value string, start int, allowBEL bool) int {
	for index := start; index < len(value); index++ {
		if allowBEL && value[index] == 0x07 {
			return index + 1
		}
		if value[index] == 0x1b && index+1 < len(value) && value[index+1] == '\\' {
			return index + 2
		}
	}
	return len(value)
}

func SanitizeCell(value string) string {
	value = Sanitize(value)
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\t", " ")
	return strings.Join(strings.Fields(value), " ")
}

func HasTerminalControl(value string) bool {
	return Sanitize(value) != value
}
