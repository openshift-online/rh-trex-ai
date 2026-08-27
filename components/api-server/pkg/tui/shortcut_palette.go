package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const (
	maxShortcutRows = 6
	shortcutGap     = 2
)

// ShortcutPalette owns all packing and responsive-elision policy for the
// contextual shortcut area in the shared header.
type ShortcutPalette struct {
	shortcuts   []ShortcutHint
	rows        int
	columns     int
	columnWidth int
	keyWidth    int
	hidden      int
}

func LayoutShortcutPalette(shortcuts []ShortcutHint, width, maxRows int) ShortcutPalette {
	width = max(0, width)
	maxRows = min(maxShortcutRows, max(0, maxRows))
	palette := ShortcutPalette{hidden: len(shortcuts)}
	if width == 0 || maxRows == 0 || len(shortcuts) == 0 {
		return palette
	}
	for _, shortcut := range shortcuts {
		if shortcut.ID == KeyHelp && ansi.StringWidth(shortcut.Text()) > width {
			return palette
		}
	}

	visible := make([]ShortcutHint, 0, len(shortcuts))
	for _, shortcut := range shortcuts {
		if shortcut.Key == "" || shortcut.Description == "" || ansi.StringWidth(shortcut.Text()) > width {
			continue
		}
		visible = append(visible, shortcut)
	}

	for len(visible) > 0 {
		candidate := packShortcutPalette(visible, width, maxRows)
		if candidate.rows > 0 {
			candidate.hidden = len(shortcuts) - len(visible)
			return candidate
		}
		remove := 0
		for index := 1; index < len(visible); index++ {
			if visible[index].Priority < visible[remove].Priority ||
				(visible[index].Priority == visible[remove].Priority && visible[index].Order > visible[remove].Order) {
				remove = index
			}
		}
		visible = append(visible[:remove], visible[remove+1:]...)
	}
	return palette
}

func packShortcutPalette(shortcuts []ShortcutHint, width, maxRows int) ShortcutPalette {
	rows := min(maxRows, len(shortcuts))
	columns := (len(shortcuts) + rows - 1) / rows
	keyWidth := 0
	for _, shortcut := range shortcuts {
		keyWidth = max(keyWidth, shortcutKeyWidth(shortcut))
	}
	columnWidth := 0
	for _, shortcut := range shortcuts {
		columnWidth = max(columnWidth, shortcutEntryWidth(shortcut, keyWidth))
	}
	total := columns*columnWidth + shortcutGap*max(0, columns-1)
	if total > width {
		return ShortcutPalette{}
	}
	return ShortcutPalette{
		shortcuts: append([]ShortcutHint(nil), shortcuts...),
		rows:      rows, columns: columns, columnWidth: columnWidth, keyWidth: keyWidth,
	}
}

func shortcutKeyWidth(shortcut ShortcutHint) int {
	return ansi.StringWidth("<" + shortcut.Key + ">")
}

func shortcutEntryWidth(shortcut ShortcutHint, keyWidth int) int {
	return keyWidth + 1 + ansi.StringWidth(shortcut.Description)
}

func (palette ShortcutPalette) Rows() int { return palette.rows }

func (palette ShortcutPalette) Hidden() int { return palette.hidden }

func (palette ShortcutPalette) ColumnWidth() int { return palette.columnWidth }

func (palette ShortcutPalette) KeyWidth() int { return palette.keyWidth }

func (palette ShortcutPalette) Width() int {
	return palette.columns*palette.columnWidth + shortcutGap*max(0, palette.columns-1)
}

func (palette ShortcutPalette) Shortcuts() []ShortcutHint {
	return append([]ShortcutHint(nil), palette.shortcuts...)
}

func (palette ShortcutPalette) Render(theme Theme, width int) []string {
	if palette.rows == 0 {
		return nil
	}
	result := make([]string, 0, palette.rows)
	for row := 0; row < palette.rows; row++ {
		var line strings.Builder
		line.WriteString(strings.Repeat(" ", max(0, width-palette.Width())))
		for column := 0; column < palette.columns; column++ {
			index := column*palette.rows + row
			if index >= len(palette.shortcuts) {
				break
			}
			shortcut := palette.shortcuts[index]
			line.WriteString(theme.Shortcut(shortcut, palette.keyWidth))
			if column+1 < palette.columns && index+palette.rows < len(palette.shortcuts) {
				line.WriteString(strings.Repeat(" ", palette.columnWidth-shortcutEntryWidth(shortcut, palette.keyWidth)+shortcutGap))
			}
		}
		result = append(result, theme.ClampLine(line.String(), width))
	}
	return result
}
