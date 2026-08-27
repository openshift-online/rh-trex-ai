package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type DialogKind string

const (
	DialogHelp         DialogKind = "help"
	DialogChoice       DialogKind = "choice"
	DialogConfirmation DialogKind = "confirmation"
	DialogForm         DialogKind = "form"
	DialogError        DialogKind = "error"
)

type Dialog interface {
	Kind() DialogKind
	Title() string
	Content(Theme) string
	Footer(Theme) string
}

type SizedDialog interface {
	SetSize(width, height int)
}

type StaticDialog struct {
	DialogKind    DialogKind
	DialogTitle   string
	DialogContent string
	DialogFooter  string
}

func (dialog StaticDialog) Kind() DialogKind { return dialog.DialogKind }
func (dialog StaticDialog) Title() string    { return dialog.DialogTitle }
func (dialog StaticDialog) Content(Theme) string {
	return dialog.DialogContent
}
func (dialog StaticDialog) Footer(Theme) string { return dialog.DialogFooter }

type ModalHost struct{ dialog Dialog }

func (host *ModalHost) Open(dialog Dialog) { host.dialog = dialog }
func (host *ModalHost) Close()             { host.dialog = nil }
func (host *ModalHost) Active() bool       { return host.dialog != nil }
func (host *ModalHost) Dialog() Dialog     { return host.dialog }

func (host ModalHost) Render(base string, width, height int, theme Theme) string {
	base = fitBlock(base, width, height, theme)
	if host.dialog == nil || width <= 0 || height <= 0 {
		return base
	}
	horizontalMargin := min(5, max(1, width/10))
	maxDialogWidth := max(1, width-horizontalMargin*2)
	if sized, present := host.dialog.(SizedDialog); present {
		sized.SetSize(max(1, maxDialogWidth-2), max(1, height-2))
	}
	content := host.dialog.Content(theme)
	if footer := host.dialog.Footer(theme); footer != "" {
		content += "\n\n" + footer
	}
	title := host.dialog.Title()
	if host.dialog.Kind() == DialogConfirmation {
		title = "<" + title + ">"
	}
	dialogWidth := min(max(12, ansi.StringWidth(title)+4), maxDialogWidth)
	for _, line := range strings.Split(content, "\n") {
		dialogWidth = min(max(dialogWidth, ansi.StringWidth(line)+4), maxDialogWidth)
	}
	dialogHeight := min(max(3, strings.Count(content, "\n")+3), height)
	dialog := theme.Frame(title, PageReady, content, dialogWidth, dialogHeight)
	if host.dialog.Kind() == DialogError {
		dialog = theme.ErrorFrame(title, content, dialogWidth, dialogHeight)
	}
	return overlayBlock(base, dialog, width, height, theme)
}

func overlayBlock(base, overlay string, width, height int, theme Theme) string {
	baseLines := strings.Split(fitBlock(base, width, height, theme), "\n")
	overlayLines := strings.Split(overlay, "\n")
	top := max(0, (height-len(overlayLines))/2)
	overlayWidth := 0
	for _, line := range overlayLines {
		overlayWidth = max(overlayWidth, ansi.StringWidth(line))
	}
	left := max(0, (width-overlayWidth)/2)
	for index, line := range overlayLines {
		row := top + index
		if row >= len(baseLines) {
			break
		}
		line = theme.ClampLine(line, min(overlayWidth, width-left))
		baseLines[row] = ansi.Cut(baseLines[row], 0, left) + line + ansi.Cut(baseLines[row], min(width, left+ansi.StringWidth(line)), width)
		baseLines[row] = theme.ClampLine(baseLines[row], width)
	}
	return strings.Join(baseLines, "\n")
}
