package tui

import "github.com/charmbracelet/x/ansi"

const headerRegionGap = 4

// ShellLayout is the sole authority for terminal-space allocation. It is
// continuous: no width is treated as a different application mode.
type ShellLayout struct {
	Width, Height   int
	HeaderRows      int
	HeaderLeftWidth int
	HeaderGap       int
	ShortcutWidth   int
	CommandRows     int
	PageRows        int
	BreadcrumbRows  int
	AlertRows       int
	ContentWidth    int
	ContentHeight   int
	ShortcutPalette ShortcutPalette
}

func CalculateShellLayout(width, height int, commandActive bool, headerLines []string, shortcuts []ShortcutHint) ShellLayout {
	result := ShellLayout{Width: max(0, width), Height: max(0, height)}
	remaining := result.Height
	reserve := func(target *int) {
		if remaining > 0 {
			*target = 1
			remaining--
		}
	}
	reserve(&result.AlertRows)
	reserve(&result.BreadcrumbRows)
	if commandActive {
		result.CommandRows = min(3, remaining)
		remaining -= result.CommandRows
	}
	if len(headerLines) > 0 || len(shortcuts) > 0 {
		reserve(&result.HeaderRows)
	}

	maxHeaderRows := result.HeaderRows
	if maxHeaderRows > 0 {
		maxHeaderRows += min(maxShortcutRows-maxHeaderRows, max(0, remaining-1))
	}
	result.HeaderLeftWidth, result.HeaderGap, result.ShortcutWidth = calculateHeaderWidths(result.Width, headerLines, shortcuts)
	result.ShortcutPalette = LayoutShortcutPalette(shortcuts, result.ShortcutWidth, maxHeaderRows)
	visibleLeftRows := min(len(headerLines), maxHeaderRows)
	headerRows := max(visibleLeftRows, result.ShortcutPalette.Rows())
	if result.HeaderRows > 0 {
		headerRows = max(1, headerRows)
		extraRows := max(0, headerRows-result.HeaderRows)
		result.HeaderRows += extraRows
		remaining -= extraRows
	}

	result.PageRows = max(0, remaining)
	result.ContentWidth = max(0, result.Width-2)
	result.ContentHeight = max(0, result.PageRows-2)
	return result
}

func calculateHeaderWidths(width int, headerLines []string, shortcuts []ShortcutHint) (left, gap, right int) {
	width = max(0, width)
	for _, line := range headerLines {
		left = max(left, ansi.StringWidth(line))
	}
	left = min(left, width)
	if len(shortcuts) == 0 {
		return left, 0, max(0, width-left)
	}

	minimumShortcutWidth := 0
	for _, shortcut := range shortcuts {
		entryWidth := ansi.StringWidth(shortcut.Text())
		if minimumShortcutWidth == 0 || entryWidth < minimumShortcutWidth {
			minimumShortcutWidth = entryWidth
		}
		if shortcut.ID == KeyHelp {
			minimumShortcutWidth = entryWidth
			break
		}
	}
	if left > 0 && width > minimumShortcutWidth {
		left = min(left, max(1, width-minimumShortcutWidth-headerRegionGap))
		gap = min(headerRegionGap, max(0, width-left-minimumShortcutWidth))
	}
	right = max(0, width-left-gap)
	return left, gap, right
}
