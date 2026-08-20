package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const breadcrumbGap = 1

type BreadcrumbSegment struct {
	Label    string
	Identity string
}

func (segment BreadcrumbSegment) Text() string {
	label := strings.ToLower(SanitizeCell(segment.Label))
	if identity := SanitizeCell(segment.Identity); identity != "" {
		label += "[" + identity + "]"
	}
	return label
}

// RenderBreadcrumb owns badge composition and responsive ancestor elision.
// It works newest-to-oldest so the active location survives constrained widths.
func RenderBreadcrumb(segments []BreadcrumbSegment, theme Theme, width int) string {
	width = max(0, width)
	if width == 0 || len(segments) == 0 {
		return theme.ClampLine("", width)
	}

	badges := make([]string, len(segments))
	for index, segment := range segments {
		badges[index] = theme.BreadcrumbBadge(segment.Text(), index == len(segments)-1)
	}
	active := badges[len(badges)-1]
	if ansi.StringWidth(active) > width {
		return theme.ClampLine("", width)
	}

	first := len(badges) - 1
	used := ansi.StringWidth(active)
	for index := len(badges) - 2; index >= 0; index-- {
		next := ansi.StringWidth(badges[index]) + breadcrumbGap
		if used+next > width {
			break
		}
		first = index
		used += next
	}
	return theme.ClampLine(strings.Join(badges[first:], strings.Repeat(" ", breadcrumbGap)), width)
}
