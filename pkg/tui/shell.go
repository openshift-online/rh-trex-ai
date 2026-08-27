package tui

import (
	"net/url"
	"strings"
	"time"
)

type headerLineKind uint8

const (
	headerService headerLineKind = iota
	headerServer
	headerStatus
)

type headerLine struct {
	key   string
	value string
	kind  headerLineKind
}

type HeaderModel struct {
	Service       string
	Origin        string
	Authenticated bool
	Scope         string
	Refreshing    bool
	LastSuccess   time.Time
	Now           time.Time
}

type ShellView struct {
	Header     HeaderModel
	Page       Page
	Command    *CommandPromptView
	Breadcrumb []BreadcrumbSegment
	HintIDs    []BindingID
}

type Shell struct {
	Theme  Theme
	Keys   KeyRegistry
	Alerts AlertManager
	Modal  ModalHost
}

func NewShell(token string) Shell {
	return Shell{Theme: DefaultTheme(), Keys: DefaultKeyRegistry(), Alerts: NewAlertManager(token)}
}

func (shell *Shell) shortcuts(view ShellView) []ShortcutHint {
	var actions []LocalAction
	if view.Page != nil && view.Command == nil && !shell.Modal.Active() {
		actions = view.Page.Actions()
	}
	return shell.Keys.Shortcuts(view.HintIDs, actions)
}

func (shell *Shell) Layout(view ShellView, width, height int) ShellLayout {
	lines := buildHeaderLines(view.Header)
	return CalculateShellLayout(width, height, view.Command != nil, headerLineValues(lines), shell.shortcuts(view))
}

func (shell *Shell) Render(view ShellView, width, height int) string {
	if view.Page == nil {
		view.Page = SemanticPage{PageTitle: "Unavailable", PageState: PageFatal, PageContent: "No page is available"}
	}
	layout := shell.Layout(view, width, height)
	rows := make([]string, 0, layout.Height)
	if layout.HeaderRows > 0 {
		rows = append(rows, renderHeader(view.Header, layout, shell.Theme)...)
	}
	if layout.CommandRows > 0 && view.Command != nil {
		prompt := shell.Theme.CommandPrompt(*view.Command, layout.Width, layout.CommandRows)
		rows = append(rows, strings.Split(prompt, "\n")...)
	}
	if layout.PageRows > 0 {
		pageBody := shell.Modal.Render(view.Page.Content(), layout.ContentWidth, layout.ContentHeight, shell.Theme)
		framed := shell.Theme.ResourceFrame(pageFrameTitle(view.Page), view.Page.State(), pageBody, layout.Width, layout.PageRows)
		rows = append(rows, strings.Split(framed, "\n")...)
	}
	if layout.BreadcrumbRows > 0 {
		rows = append(rows, RenderBreadcrumb(view.Breadcrumb, shell.Theme, layout.Width))
	}
	for len(rows) < layout.Height-layout.AlertRows {
		rows = append(rows, shell.Theme.ClampLine("", layout.Width))
	}
	if layout.AlertRows > 0 {
		alertLine := ""
		if alert, present := shell.Alerts.Active(); present {
			alertLine = shell.Theme.Alert(alert.Severity, alertPrefix(alert.Severity)+": "+alert.Summary)
		}
		rows = append(rows, shell.Theme.ClampLine(alertLine, layout.Width))
	}
	if len(rows) > layout.Height {
		rows = rows[:layout.Height]
	}
	return strings.Join(rows, "\n")
}

func renderHeader(header HeaderModel, layout ShellLayout, theme Theme) []string {
	leftLines := buildHeaderLines(header)
	rightLines := layout.ShortcutPalette.Render(theme, layout.ShortcutWidth)
	rows := make([]string, 0, layout.HeaderRows)
	for row := 0; row < layout.HeaderRows; row++ {
		var line strings.Builder
		if layout.HeaderLeftWidth > 0 {
			left := ""
			if headerLine, present := headerLineAt(leftLines, row, layout.HeaderRows); present {
				left = theme.HeaderKey(headerLine.key+":") + " "
				switch headerLine.kind {
				case headerService:
					left += theme.Standard(headerLine.value)
				case headerServer:
					left += theme.Emphasis(headerLine.value)
				default:
					left += theme.Subtle(headerLine.value)
				}
			}
			line.WriteString(theme.ClampLine(left, layout.HeaderLeftWidth))
		}
		line.WriteString(strings.Repeat(" ", layout.HeaderGap))
		if layout.ShortcutWidth > 0 {
			right := ""
			if row < len(rightLines) {
				right = rightLines[row]
			}
			line.WriteString(theme.ClampLine(right, layout.ShortcutWidth))
		}
		rows = append(rows, theme.ClampLine(line.String(), layout.Width))
	}
	return rows
}

func buildHeaderLines(header HeaderModel) []headerLine {
	var lines []headerLine
	if service := SanitizeCell(header.Service); service != "" {
		lines = append(lines, headerLine{key: "Service", value: service, kind: headerService})
	}
	if raw := strings.TrimSpace(header.Origin); raw != "" {
		if parsed, err := url.Parse(raw); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			lines = append(lines, headerLine{key: "Context", value: SanitizeCell(parsed.Scheme + "://" + parsed.Host), kind: headerServer})
		}
	}

	var status []string
	if header.Authenticated {
		status = append(status, "authenticated")
	} else {
		status = append(status, "anonymous")
	}
	if header.Scope != "" {
		status = append(status, SanitizeCell(header.Scope))
	}
	if header.Refreshing {
		status = append(status, "refreshing…")
	} else if !header.LastSuccess.IsZero() {
		now := header.Now
		if now.IsZero() {
			now = time.Now()
		}
		age := max(time.Duration(0), now.Sub(header.LastSuccess)).Round(time.Second)
		status = append(status, "refreshed "+age.String()+" ago")
	}
	if len(status) > 0 {
		lines = append(lines, headerLine{key: "Status", value: strings.Join(status, " · "), kind: headerStatus})
	}
	return lines
}

func headerLineAt(lines []headerLine, row, rows int) (headerLine, bool) {
	if rows < 3 {
		if row >= 0 && row < len(lines) {
			return lines[row], true
		}
		return headerLine{}, false
	}
	for _, line := range lines {
		target := 0
		switch line.kind {
		case headerServer:
			target = rows - 2
		case headerStatus:
			target = rows - 1
		}
		if row == target {
			return line, true
		}
	}
	return headerLine{}, false
}

func headerLineValues(lines []headerLine) []string {
	values := make([]string, 0, len(lines))
	for _, line := range lines {
		values = append(values, line.key+": "+line.value)
	}
	return values
}
