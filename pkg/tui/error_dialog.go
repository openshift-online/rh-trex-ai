package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

const maxCompactErrorWidth = 60

type ErrorDialog struct {
	title          string
	summary        string
	context        string
	details        string
	wrappedSummary string
	wrappedContext string
	viewport       viewport.Model
	keys           KeyRegistry
	expanded       bool
	focus          int
	width          int
}

func NewErrorDialog(title, summary, context string, alert Alert, keys KeyRegistry) *ErrorDialog {
	details := alert.Details
	if details == "" {
		details = alert.Summary
	}
	dialog := &ErrorDialog{
		title: SanitizeCell(title), summary: SanitizeCell(summary), context: SanitizeCell(context), details: details,
		viewport: viewport.New(72, 10), keys: keys,
	}
	dialog.SetSize(72, 16)
	return dialog
}

func (dialog *ErrorDialog) Kind() DialogKind { return DialogError }
func (dialog *ErrorDialog) Title() string    { return dialog.title }
func (dialog *ErrorDialog) Expanded() bool   { return dialog.expanded }

func (dialog *ErrorDialog) Content(theme Theme) string {
	if dialog.expanded {
		return theme.Negative(dialog.wrappedSummary) + "\n\n" + theme.Standard(dialog.viewport.View())
	}
	buttons := renderDialogButtons(theme, "Close", "Details", dialog.focus, 2)
	summaryLines := strings.Split(dialog.wrappedSummary, "\n")
	contextLines := strings.Split(dialog.wrappedContext, "\n")
	contentWidth := ansi.StringWidth(buttons)
	for _, line := range append(append([]string(nil), summaryLines...), contextLines...) {
		contentWidth = max(contentWidth, ansi.StringWidth(line))
	}
	lines := make([]string, 0, len(summaryLines)+len(contextLines)+2)
	for _, line := range summaryLines {
		lines = append(lines, centerDialogLine(theme.Negative(line), contentWidth))
	}
	if dialog.wrappedContext != "" {
		for _, line := range contextLines {
			lines = append(lines, centerDialogLine(theme.FieldMetadata(line), contentWidth))
		}
	}
	lines = append(lines, "", centerDialogLine(buttons, contentWidth))
	return strings.Join(lines, "\n")
}

func (dialog *ErrorDialog) Footer(theme Theme) string {
	if !dialog.expanded {
		return ""
	}
	hints := dialog.keys.Hints(KeyScrollUp, KeyScrollDown, KeyPageUp, KeyPageDown, KeyScrollHome, KeyScrollEnd, KeyCancel)
	return theme.FieldMetadata(hints + fmt.Sprintf("  %d%%", int(dialog.viewport.ScrollPercent()*100)))
}

func (dialog *ErrorDialog) SetSize(width, height int) {
	width = max(1, width)
	dialog.width = width
	compactWidth := min(width, maxCompactErrorWidth)
	dialog.wrappedSummary = ansi.Wrap(dialog.summary, compactWidth, " ")
	dialog.wrappedContext = ansi.Wrap(dialog.context, compactWidth, " ")
	summaryRows := strings.Count(dialog.wrappedSummary, "\n") + 1
	viewportHeight := max(1, height-summaryRows-3)
	wasAtBottom := dialog.viewport.TotalLineCount() > 0 && dialog.viewport.AtBottom()
	previousOffset := dialog.viewport.YOffset
	dialog.viewport.Width = width
	dialog.viewport.Height = viewportHeight
	dialog.viewport.SetContent(ansi.Hardwrap(dialog.details, width, true))
	if wasAtBottom {
		dialog.viewport.GotoBottom()
	} else {
		dialog.viewport.SetYOffset(previousOffset)
	}
}

func (dialog *ErrorDialog) Update(message tea.Msg) (closeDialog bool, command tea.Cmd) {
	key, isKey := message.(tea.KeyMsg)
	if !dialog.expanded {
		if !isKey {
			return false, nil
		}
		switch {
		case dialog.keys.Matches(key, KeyCancel):
			return true, nil
		case dialog.keys.Matches(key, KeyPreviousFocus), dialog.keys.Matches(key, KeyChoicePrevious):
			dialog.focus = 0
		case dialog.keys.Matches(key, KeyNextFocus), dialog.keys.Matches(key, KeyChoiceNext):
			dialog.focus = 1
		case dialog.keys.Matches(key, KeySubmit):
			if dialog.focus == 0 {
				return true, nil
			}
			dialog.expanded = true
			dialog.viewport.GotoTop()
		}
		return false, nil
	}

	if isKey {
		switch {
		case dialog.keys.Matches(key, KeyCancel):
			dialog.expanded = false
			return false, nil
		case dialog.keys.Matches(key, KeyScrollUp):
			dialog.viewport.ScrollUp(1)
			return false, nil
		case dialog.keys.Matches(key, KeyScrollDown):
			dialog.viewport.ScrollDown(1)
			return false, nil
		case dialog.keys.Matches(key, KeyPageUp):
			dialog.viewport.PageUp()
			return false, nil
		case dialog.keys.Matches(key, KeyPageDown):
			dialog.viewport.PageDown()
			return false, nil
		case dialog.keys.Matches(key, KeyScrollHome):
			dialog.viewport.GotoTop()
			return false, nil
		case dialog.keys.Matches(key, KeyScrollEnd):
			dialog.viewport.GotoBottom()
			return false, nil
		}
	}
	dialog.viewport, command = dialog.viewport.Update(message)
	return false, command
}
