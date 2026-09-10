package tui

import (
	"strings"
	"testing"
)

func TestSanitizeTerminalInjection(t *testing.T) {
	payloads := []struct {
		name  string
		value string
		want  string
	}{
		{"CSI", "safe\x1b[31m red\x1b[0m", "safe red"},
		{"OSC title", "safe\x1b]0;owned\x07 text", "safe text"},
		{"OSC clipboard", "safe\x1b]52;c;dG9rZW4=\x1b\\ text", "safe text"},
		{"DCS", "safe\x1bPmalicious\x1b\\ text", "safe text"},
		{"SOS", "safe\x1bXmalicious\x1b\\ text", "safe text"},
		{"PM", "safe\x1b^malicious\x1b\\ text", "safe text"},
		{"APC", "safe\x1b_malicious\x1b\\ text", "safe text"},
		{"C0 C1 DEL", "a\x00\x07\x7f\u0085b", "ab"},
		{"framework", `before["region"]after[""]`, "beforeafter"},
		{"malformed", "safe\x1b]unterminated", "safe"},
		{"unicode", "héllo 世界 🚀", "héllo 世界 🚀"},
	}
	for _, test := range payloads {
		t.Run(test.name, func(t *testing.T) {
			if got := Sanitize(test.value); got != test.want {
				t.Fatalf("Sanitize() = %q, want %q", got, test.want)
			}
			if got := Sanitize(Sanitize(test.value)); got != test.want {
				t.Fatalf("Sanitize() is not idempotent: %q", got)
			}
		})
	}
}

func TestSanitizeCellFlattensLayoutControls(t *testing.T) {
	got := SanitizeCell("first\n\tsecond   third")
	if got != "first second third" || strings.ContainsAny(got, "\n\t") {
		t.Fatalf("SanitizeCell() = %q", got)
	}
}
