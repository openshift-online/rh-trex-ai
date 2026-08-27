package tui

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestContinuousLayoutNeverProducesNegativeDimensions(t *testing.T) {
	shortcuts := DefaultKeyRegistry().Shortcuts([]BindingID{KeyHelp, KeyCommand, KeyFilter, KeyCancel}, nil)
	for _, size := range [][2]int{{120, 40}, {48, 12}, {8, 4}, {1, 1}, {0, 0}, {-5, -2}} {
		layout := CalculateShellLayout(size[0], size[1], true, []string{"server", "service", "status"}, shortcuts)
		values := []int{layout.Width, layout.Height, layout.HeaderRows, layout.CommandRows, layout.PageRows, layout.BreadcrumbRows, layout.AlertRows, layout.ContentWidth, layout.ContentHeight}
		for _, value := range values {
			if value < 0 {
				t.Fatalf("layout for %v contains negative value: %#v", size, layout)
			}
		}
		if layout.HeaderRows+layout.CommandRows+layout.PageRows+layout.BreadcrumbRows+layout.AlertRows != layout.Height {
			t.Fatalf("layout for %v does not consume terminal height: %#v", size, layout)
		}
	}
	active := CalculateShellLayout(80, 20, true, []string{"service", "context", "status"}, shortcuts)
	inactive := CalculateShellLayout(80, 20, false, []string{"service", "context", "status"}, shortcuts)
	if active.CommandRows != 3 || inactive.CommandRows != 0 || inactive.PageRows-active.PageRows != 3 {
		t.Fatalf("prompt allocation did not return exactly three rows: active %#v, inactive %#v", active, inactive)
	}
	short := CalculateShellLayout(20, 3, true, []string{"service"}, shortcuts)
	if short.CommandRows != 1 || short.AlertRows != 1 || short.BreadcrumbRows != 1 {
		t.Fatalf("extremely short fallback allocation = %#v", short)
	}
}

func TestShortcutPaletteUsesRegistryOrderAndResponsivePriority(t *testing.T) {
	registry := DefaultKeyRegistry()
	actions := []LocalAction{{Label: "archive", Hotkey: "x"}}
	shortcuts := registry.Shortcuts([]BindingID{
		KeySortDirection, KeyQuit, KeyHelp, KeyCommand, KeyFilter, KeyCancel,
		KeyNavigate, KeyDetail, KeyActions, KeySortNext,
	}, actions)
	if shortcuts[0].ID != KeyQuit || shortcuts[1].ID != KeyHelp || shortcuts[len(shortcuts)-1].Key != "x" {
		t.Fatalf("shortcut registry order = %#v", shortcuts)
	}

	spacious := LayoutShortcutPalette(shortcuts, 80, maxShortcutRows)
	if spacious.Hidden() != 0 || spacious.Rows() > maxShortcutRows || len(spacious.Shortcuts()) != len(shortcuts) {
		t.Fatalf("spacious palette = %#v", spacious)
	}
	for _, shortcut := range spacious.Shortcuts() {
		if !strings.Contains(registry.ShortcutHelp([]BindingID{
			KeySortDirection, KeyQuit, KeyHelp, KeyCommand, KeyFilter, KeyCancel,
			KeyNavigate, KeyDetail, KeyActions, KeySortNext,
		}, actions), shortcut.Text()) {
			t.Fatalf("header shortcut %q absent from help", shortcut.Text())
		}
	}

	constrained := LayoutShortcutPalette(shortcuts, 24, 2)
	if constrained.Hidden() == 0 || constrained.Rows() > 2 {
		t.Fatalf("constrained palette did not elide: %#v", constrained)
	}
	foundHelp := false
	for _, shortcut := range constrained.Shortcuts() {
		foundHelp = foundHelp || shortcut.ID == KeyHelp
	}
	if !foundHelp {
		t.Fatalf("constrained palette elided help: %#v", constrained.Shortcuts())
	}
	for _, line := range constrained.Render(PlainTheme(), 24) {
		if strings.Contains(line, "…") {
			t.Fatalf("palette rendered a partial shortcut: %q", line)
		}
	}
	if tooNarrow := LayoutShortcutPalette(shortcuts, 6, maxShortcutRows); tooNarrow.Rows() != 0 {
		t.Fatalf("palette rendered shortcuts without room for Help: %#v", tooNarrow)
	}

	alignedPalette := LayoutShortcutPalette([]ShortcutHint{
		{Key: "a", Description: "one", Order: 1},
		{Key: "bb", Description: "two", Order: 2},
		{Key: "c", Description: "three", Order: 3},
		{Key: "dd", Description: "four", Order: 4},
	}, 80, 2)
	aligned := alignedPalette.Render(PlainTheme(), 80)
	if len(aligned) != 2 || strings.Index(aligned[0], "<c>") != strings.Index(aligned[1], "<dd>") {
		t.Fatalf("shortcut columns are not aligned: %q", aligned)
	}
	if alignedPalette.KeyWidth() != len("<dd>") || alignedPalette.ColumnWidth() != len("<dd> three") ||
		!strings.HasSuffix(strings.TrimRight(aligned[0], " "), "<c>  three") {
		t.Fatalf("shortcut columns are not equal-width and right-aligned: %#v %q", alignedPalette, aligned)
	}
	leading := 80 - alignedPalette.Width()
	for index, shortcut := range alignedPalette.Shortcuts() {
		row := index % alignedPalette.Rows()
		column := index / alignedPalette.Rows()
		want := leading + column*(alignedPalette.ColumnWidth()+shortcutGap) + alignedPalette.KeyWidth() + 1
		if got := strings.Index(aligned[row], shortcut.Description); got != want {
			t.Fatalf("action %q starts at cell %d, want %d in %q", shortcut.Description, got, want, aligned[row])
		}
	}

	restored := LayoutShortcutPalette(shortcuts, 80, maxShortcutRows)
	if len(restored.Shortcuts()) != len(spacious.Shortcuts()) {
		t.Fatalf("restored palette did not recover entries: %#v", restored)
	}
	for index := range spacious.Shortcuts() {
		if restored.Shortcuts()[index].Text() != spacious.Shortcuts()[index].Text() {
			t.Fatalf("restored order changed at %d", index)
		}
	}
}

func TestShellRendersShortcutsOnlyInTopHeader(t *testing.T) {
	shell := NewShell("")
	shell.Theme = PlainTheme()
	page := SemanticPage{
		PageTitle: "Items", PageState: PageReady, PageContent: "one",
		PageActions: []LocalAction{{Label: "archive", Hotkey: "x"}},
	}
	view := ShellView{
		Header: HeaderModel{Service: "Inventory API", Origin: "https://api.example.test", Authenticated: true},
		Page:   page, Breadcrumb: []BreadcrumbSegment{{Label: "Items"}}, HintIDs: []BindingID{KeyHelp, KeyDetail, KeyQuit},
	}
	output := shell.Render(view, 48, 12)
	lines := strings.Split(output, "\n")
	if len(lines) != 12 || !strings.HasPrefix(lines[0], "Service: Inventory API") ||
		!strings.Contains(strings.Join(lines[:6], "\n"), "<?> help") ||
		!strings.Contains(strings.Join(lines[:7], "\n"), "<x> archive") {
		t.Fatalf("top shortcut palette missing:\n%s", output)
	}
	if strings.Contains(lines[0], "Items") || !strings.HasPrefix(lines[2], "Context: https://api.example.test") ||
		!strings.HasPrefix(lines[3], "Status: authenticated") || strings.TrimSpace(lines[1][:33]) != "" {
		t.Fatalf("left header region is not vertically anchored:\n%s", output)
	}
	if !strings.HasSuffix(strings.TrimRight(lines[0], " "), "<q> quit") {
		t.Fatalf("shortcut palette is not anchored to the upper-right:\n%s", output)
	}
	if got := strings.TrimSpace(lines[len(lines)-2]); got != "<items>" {
		t.Fatalf("breadcrumb row contains duplicate hints: %q\n%s", got, output)
	}
	if got := strings.TrimSpace(lines[len(lines)-1]); got != "" {
		t.Fatalf("empty alert rail moved or contains hints: %q\n%s", got, output)
	}
}

func TestHeaderLeftRegionAnchorsAndConstrainedPriority(t *testing.T) {
	lines := buildHeaderLines(HeaderModel{
		Service: "Inventory API", Origin: "https://api.example.test", Authenticated: true,
		LastSuccess: time.Date(2026, 8, 4, 11, 59, 56, 0, time.UTC),
		Now:         time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC),
	})
	want := map[int]string{0: "Inventory API", 4: "https://api.example.test", 5: "authenticated · refreshed 4s ago"}
	wantKeys := map[int]string{0: "Service", 4: "Context", 5: "Status"}
	for row := 0; row < 6; row++ {
		line, present := headerLineAt(lines, row, 6)
		expected, wanted := want[row]
		if present != wanted || present && (line.value != expected || line.key != wantKeys[row]) {
			t.Fatalf("six-row header row %d = %#v, present %v; want %q, present %v", row, line, present, expected, wanted)
		}
	}
	for row, expected := range []string{"Inventory API", "https://api.example.test"} {
		line, present := headerLineAt(lines, row, 2)
		if !present || line.value != expected {
			t.Fatalf("two-row header row %d = %#v, present %v; want %q", row, line, present, expected)
		}
	}
}

func TestModalHeaderShowsOnlyDispatchableShortcuts(t *testing.T) {
	model := &Model{
		shell: NewShell(""), mode: modeHelp,
		ResourceTableComponent: ResourceTableComponent{leftOverflow: 2, rightOverflow: 3},
	}
	keys := model.applicableKeys()
	want := []BindingID{KeyCancel, KeyHelp, KeyForceQuit}
	if len(keys) != len(want) {
		t.Fatalf("help-mode keys = %v, want %v", keys, want)
	}
	for index := range want {
		if keys[index] != want[index] {
			t.Fatalf("help-mode keys = %v, want %v", keys, want)
		}
	}
}

func TestGeneratedHeaderActionsExcludeReadOperations(t *testing.T) {
	view := View{OperationIDs: []string{"listItems", "archiveItem"}}
	model := &Model{descriptor: Descriptor{Operations: []Operation{
		{ID: "listItems", Capabilities: []string{"list"}, Presentation: ActionPresentation{Label: "reload", Hotkey: "r"}},
		{ID: "archiveItem", Capabilities: []string{"action"}, Presentation: ActionPresentation{Label: "archive", Hotkey: "x"}},
	}}}
	actions := model.localActions(view)
	if len(actions) != 1 || actions[0].Hotkey != "x" || actions[0].Label != "archive" {
		t.Fatalf("dispatchable generated actions = %#v", actions)
	}
}

func TestShellSnapshotKeepsAlertOnFinalRowAcrossTransitions(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	shell := NewShell("")
	shell.Theme = PlainTheme()
	shell.Alerts.now = func() time.Time { return now }
	shell.Alerts.Push("request", AlertError, "network unavailable")
	count := 2
	page := ResourceTablePage{SemanticPage{PageTitle: "Items", PageCount: &count, PageState: PageStale, PageContent: "NAME  STATE\none   ready\ntwo   waiting"}}
	view := ShellView{Header: HeaderModel{Service: "Inventory API", Origin: "https://api.example.test", Authenticated: true}, Page: page, Breadcrumb: []BreadcrumbSegment{{Label: "Items"}}, HintIDs: []BindingID{KeyHelp, KeyQuit}}

	assertRail := func(label string, output string, height int) {
		t.Helper()
		lines := strings.Split(output, "\n")
		if len(lines) != height {
			t.Fatalf("%s height = %d, want %d\n%s", label, len(lines), height, output)
		}
		if got := strings.TrimSpace(lines[len(lines)-1]); !strings.HasPrefix(got, "ERROR:") {
			t.Fatalf("%s final row = %q\n%s", label, got, output)
		}
	}

	spacious := shell.Render(view, 64, 12)
	assertRail("spacious", spacious, 12)
	if !strings.Contains(spacious, "Service: Inventory API") || strings.Contains(spacious, "Inventory API — Items") || !strings.Contains(spacious, "Items(all)[2] · stale") {
		t.Fatalf("spacious snapshot lost semantic chrome:\n%s", spacious)
	}

	view.Command = &CommandPromptView{Kind: CommandFilter, Input: "waiting"}
	command := shell.Render(view, 64, 12)
	assertRail("command", command, 12)
	if !strings.Contains(command, "🦕/ waiting") {
		t.Fatalf("command snapshot omitted command bar:\n%s", command)
	}

	view.Command = nil
	shell.Modal.Open(StaticDialog{DialogKind: DialogHelp, DialogTitle: "Help", DialogContent: "? help", DialogFooter: "esc close"})
	dialog := shell.Render(view, 64, 12)
	assertRail("dialog", dialog, 12)
	if !strings.Contains(dialog, "Help") || !strings.Contains(dialog, "? help") {
		t.Fatalf("dialog snapshot omitted overlay:\n%s", dialog)
	}

	narrow := shell.Render(view, 9, 5)
	assertRail("narrow", narrow, 5)
	if !strings.Contains(narrow, "Items") {
		t.Fatalf("narrow snapshot lost page identity:\n%s", narrow)
	}
}

func TestPageFrameRendersEverySemanticState(t *testing.T) {
	theme := PlainTheme()
	for _, state := range []PageState{PageReady, PageLoading, PageEmpty, PageForbidden, PageStale, PageFatal} {
		output := theme.ResourceFrame(PageFrameTitle{Kind: "Records", Context: "all"}, state, "content", 32, 5)
		if len(strings.Split(output, "\n")) != 5 || !strings.Contains(output, "Records") {
			t.Fatalf("state %s frame = %q", state, output)
		}
		if state != PageReady && !strings.Contains(output, string(state)) {
			t.Fatalf("state %s absent from frame: %q", state, output)
		}
	}
}

func TestPlainPageFrameSnapshot(t *testing.T) {
	const expected = "╭─ Items(all) ─╮\n│one           │\n│              │\n╰──────────────╯"
	if actual := PlainTheme().ResourceFrame(PageFrameTitle{Kind: "Items", Context: "all"}, PageReady, "one", 16, 4); actual != expected {
		t.Fatalf("page-frame snapshot changed\nexpected:\n%s\nactual:\n%s", expected, actual)
	}
}

func TestSimpleCatalogFrameTitleOmitsSyntheticContextAndCount(t *testing.T) {
	count := 3
	label := PlainTheme().FrameLabel(PageFrameTitle{Kind: "Resources", Context: "all", Count: &count, Simple: true}, PageReady)
	if label != "Resources" {
		t.Fatalf("simple catalog frame label = %q", label)
	}
}

func TestFrameTitleIsCenteredAndUsesSemanticSegments(t *testing.T) {
	count := 8
	title := PageFrameTitle{Kind: "Dinosaur", Context: "all", Count: &count}
	const width = 44
	frame := PlainTheme().ResourceFrame(title, PageReady, "", width, 3)
	top := strings.Split(frame, "\n")[0]
	label := " Dinosaur(all)[8] "
	wantStart := 1 + (width-2-len(label))/2
	labelByte := strings.Index(top, label)
	if labelByte < 0 {
		t.Fatalf("frame title absent: %q", top)
	}
	if got := ansi.StringWidth(top[:labelByte]); got != wantStart {
		t.Fatalf("frame title starts at cell %d, want %d: %q", got, wantStart, top)
	}

	theme := DefaultTheme()
	if theme.Primary.GetForeground() == theme.Secondary.GetForeground() ||
		theme.Secondary.GetForeground() == theme.Warning.GetForeground() {
		t.Fatal("kind, context, and count do not have distinct semantic colors")
	}
}

func TestTableStylesOverrideBubblesRowColors(t *testing.T) {
	theme := DefaultTheme()
	styles := theme.TableStyles()
	if styles.Header.GetForeground() != theme.Primary.GetForeground() {
		t.Fatalf("table header foreground = %v, want primary %v", styles.Header.GetForeground(), theme.Primary.GetForeground())
	}
	if styles.Cell.GetForeground() != (lipgloss.NoColor{}) {
		t.Fatalf("cell foreground = %v; per-cell color would override selected-row black", styles.Cell.GetForeground())
	}
	if theme.tableRowStyle().GetForeground() != styles.Selected.GetBackground() {
		t.Fatalf("unselected row foreground = %v, want selected background %v", theme.tableRowStyle().GetForeground(), styles.Selected.GetBackground())
	}
	if theme.SelectedForeground.GetForeground() != lipgloss.Color("0") {
		t.Fatalf("selected foreground token = %v, want black", theme.SelectedForeground.GetForeground())
	}
	if styles.Selected.GetForeground() != theme.SelectedForeground.GetForeground() ||
		styles.Selected.GetBackground() != theme.SelectedBackground.GetBackground() {
		t.Fatalf("selected row style = foreground %v background %v, want foreground %v background %v",
			styles.Selected.GetForeground(), styles.Selected.GetBackground(),
			theme.SelectedForeground.GetForeground(), theme.SelectedBackground.GetBackground())
	}
}

func TestDetailBodyMatchesSharedACPFormatting(t *testing.T) {
	theme := DefaultTheme()
	if theme.DetailKey.GetForeground() != lipgloss.Color("240") {
		t.Fatalf("detail key foreground = %v, want dim 240", theme.DetailKey.GetForeground())
	}
	if theme.DetailValue.GetForeground() != lipgloss.Color("255") || theme.detailBodyStyle().GetForeground() != theme.DetailValue.GetForeground() {
		t.Fatalf("detail value foreground = %v and body foreground = %v, want bright white 255", theme.DetailValue.GetForeground(), theme.detailBodyStyle().GetForeground())
	}

	value := map[string]any{"id": float64(7), "species": "tyrannosaurus rex"}
	const want = "     id  7\nspecies  tyrannosaurus\n         rex"
	if got := renderDetailBody(value, PlainTheme(), 24); got != want {
		t.Fatalf("detail body:\n%q\nwant:\n%q", got, want)
	}
	wide := renderDetailBody(value, PlainTheme(), 40)
	narrow := renderDetailBody(value, PlainTheme(), 16)
	if wide == narrow || !strings.Contains(narrow, "\n       urus rex") {
		t.Fatalf("detail body did not reflow with aligned continuation: wide=%q narrow=%q", wide, narrow)
	}
}

func TestRawJSONHighlightingIsTokenAwareAndLossless(t *testing.T) {
	raw := "{\n  \"name\": \"Rex\",\n  \"count\": 7,\n  \"active\": true,\n  \"other\": null\n}"
	tokens := tokenizeRawJSON(raw)
	wantKinds := map[string]rawJSONTokenKind{
		"\"name\"": rawJSONKey,
		"\"Rex\"":  rawJSONString,
		"7":        rawJSONNumber,
		"true":     rawJSONLiteral,
		"null":     rawJSONLiteral,
		"{":        rawJSONPunctuation,
	}
	for text, want := range wantKinds {
		found := false
		for _, token := range tokens {
			if token.text == text {
				found = true
				if token.kind != want {
					t.Fatalf("token %q kind = %v, want %v", text, token.kind, want)
				}
				break
			}
		}
		if !found {
			t.Fatalf("token %q absent from %#v", text, tokens)
		}
	}
	if got := ansi.Strip(renderRawCode(raw, DefaultTheme())); got != raw {
		t.Fatalf("highlighting changed raw JSON:\n%q\nwant:\n%q", got, raw)
	}
	theme := DefaultTheme()
	colors := []any{
		theme.RawCodeKey.GetForeground(),
		theme.RawCodeString.GetForeground(),
		theme.RawCodeNumber.GetForeground(),
		theme.RawCodeLiteral.GetForeground(),
		theme.RawCodePunctuation.GetForeground(),
	}
	seen := make(map[any]bool, len(colors))
	for _, color := range colors {
		seen[color] = true
	}
	if len(seen) != len(colors) {
		t.Fatalf("raw code token colors are not distinct: %#v", colors)
	}
}

func TestFrameTitleAppendsSanitizedFilterBadgeAndFilteredCount(t *testing.T) {
	count := 2
	title := PageFrameTitle{Kind: "Dinosaur", Context: "all", Count: &count, Filter: "arch\x1b]52;c;bad\a ae"}
	label := PlainTheme().FrameLabel(title, PageReady)
	if label != "Dinosaur(all)[2] </arch ae>" {
		t.Fatalf("filtered frame label = %q", label)
	}
	frame := PlainTheme().ResourceFrame(title, PageReady, "", 52, 3)
	if !strings.Contains(strings.Split(frame, "\n")[0], "Dinosaur(all)[2] </arch ae>") {
		t.Fatalf("filter badge absent from centered frame title:\n%s", frame)
	}
	if got := PlainTheme().FrameLabel(PageFrameTitle{Kind: "Dinosaur", Context: "all", Count: &count}, PageReady); strings.Contains(got, "</") {
		t.Fatalf("cleared filter retained badge: %q", got)
	}
	if _, absent := DefaultTheme().FilterBadge.GetBackground().(lipgloss.NoColor); absent {
		t.Fatal("filter badge has no distinct semantic background")
	}
}

func TestCommandPromptUsesCompleteModeSpecificBorder(t *testing.T) {
	theme := PlainTheme()
	resource := theme.CommandPrompt(CommandPromptView{Kind: CommandResource, Input: "dinosaurs"}, 28, 3)
	lines := strings.Split(resource, "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "┌") || !strings.HasSuffix(lines[0], "┐") ||
		!strings.HasPrefix(lines[1], "│ 🦖> dinosaurs") || !strings.HasSuffix(lines[1], "│") ||
		!strings.HasPrefix(lines[2], "└") || !strings.HasSuffix(lines[2], "┘") {
		t.Fatalf("resource prompt is not fully bordered:\n%s", resource)
	}
	filter := theme.CommandPrompt(CommandPromptView{Kind: CommandFilter, Input: "search"}, 28, 3)
	if !strings.Contains(filter, "🦕/ search") {
		t.Fatalf("filter prompt lacks mode-specific prompt UI:\n%s", filter)
	}
	bar := NewCommandBar()
	bar.Begin(CommandResource, "d")
	actual := theme.CommandPrompt(bar.View(theme, 28), 28, 3)
	if middle := strings.Split(actual, "\n")[1]; strings.Count(middle, ">") != 1 {
		t.Fatalf("resource prompt contains a duplicate widget prefix: %q", middle)
	}
	defaultTheme := DefaultTheme()
	if defaultTheme.CommandBorder.GetForeground() == defaultTheme.FilterBorder.GetForeground() {
		t.Fatal("resource and filter prompts share one border color")
	}
}

func TestBreadcrumbBadgesPreserveActiveSegmentWhenConstrained(t *testing.T) {
	segments := []BreadcrumbSegment{
		{Label: "Projects"},
		{Label: "Agents"},
		{Label: "Sessions", Identity: "ABC-7"},
	}
	wide := strings.TrimRight(RenderBreadcrumb(segments, PlainTheme(), 80), " ")
	if wide != " <projects>   <agents>   <sessions[ABC-7]>" {
		t.Fatalf("wide breadcrumb = %q", wide)
	}
	active := " <sessions[ABC-7]> "
	narrow := strings.TrimRight(RenderBreadcrumb(segments, PlainTheme(), len(active)), " ")
	if narrow != strings.TrimRight(active, " ") || strings.Contains(narrow, "agents") || strings.Contains(narrow, "projects") {
		t.Fatalf("constrained breadcrumb did not preserve only active badge: %q", narrow)
	}
	if got := strings.TrimSpace(RenderBreadcrumb(segments, PlainTheme(), len(active)-1)); got != "" {
		t.Fatalf("partial active badge rendered: %q", got)
	}

	theme := DefaultTheme()
	if theme.BreadcrumbAncestor.GetBackground() == theme.BreadcrumbActive.GetBackground() ||
		theme.BreadcrumbAncestor.GetForeground() == theme.BreadcrumbActive.GetForeground() {
		t.Fatal("ancestor and active breadcrumb badges do not have distinct semantic styles")
	}
}

func TestAlertSeverityLifetimePriorityAndRedaction(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	manager := NewAlertManager("top-secret")
	manager.now = func() time.Time { return now }
	manager.Push("info", AlertInfo, "connected")
	manager.Push("warning", AlertWarning, "slow response")
	manager.Push("error", AlertError, "Bearer abc.def and top-secret failed")
	manager.Push("success", AlertSuccess, "recovered")
	active, present := manager.Active()
	if !present || active.Severity != AlertError || strings.Contains(active.Summary, "abc.def") || strings.Contains(active.Summary, "top-secret") {
		t.Fatalf("active alert = %#v, present %v", active, present)
	}
	manager.Clear("error")
	active, _ = manager.Active()
	if active.Severity != AlertWarning {
		t.Fatalf("warning did not outrank transient alerts: %#v", active)
	}
	manager.Clear("warning")
	now = now.Add(alertLifetime + time.Second)
	if _, present := manager.Active(); present {
		t.Fatalf("transient alerts did not expire: %#v", manager.alerts)
	}
}

func TestBackgroundAPIErrorRemainsInRailWithoutStealingFocus(t *testing.T) {
	model := &Model{mode: modeBrowse, shell: NewShell("secret-token")}
	body := "  first response line\nlast response line  \n"
	requestErr := &APIError{
		OperationID: "listThings", Method: "GET", URL: "https://example.test/things",
		Status: 503, Body: body,
	}
	model.presentRequestError("refresh:1", requestErr, true)
	if model.mode != modeBrowse || model.errorDialog != nil || model.shell.Modal.Active() {
		t.Fatalf("background API error stole focus: mode=%v dialog=%#v modal=%v", model.mode, model.errorDialog, model.shell.Modal.Active())
	}
	alert, present := model.shell.Alerts.Active()
	if !present || alert.Severity != AlertError || !strings.HasSuffix(alert.Details, body) {
		t.Fatalf("background API error did not retain full whitespace: %#v", alert)
	}
}

func TestAPIErrorUsesTRexEnvelopeWithSafeGenericFallback(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "https://user:password@example.test/things?token=secret#fragment", nil)
	if err != nil {
		t.Fatal(err)
	}
	envelope := newAPIError(
		"listThings", request, &http.Response{StatusCode: http.StatusUnprocessableEntity},
		[]byte(`{"kind":"Error","reason":"Unable to load things","code":"rh-trex-ai-1"}`), nil,
	)
	if envelope.DialogTitle() != "Error" || envelope.DialogMessage() != "Unable to load things" || !strings.Contains(envelope.DialogContext(), "rh-trex-ai-1") {
		t.Fatalf("TRex envelope presentation = title %q message %q context %q", envelope.DialogTitle(), envelope.DialogMessage(), envelope.DialogContext())
	}
	for _, forbidden := range []string{"user", "password", "token=secret", "fragment"} {
		if strings.Contains(envelope.Details(), forbidden) {
			t.Fatalf("safe API error URL retained %q: %q", forbidden, envelope.Details())
		}
	}

	fallback := newAPIError("listThings", request, &http.Response{StatusCode: http.StatusBadGateway}, []byte("upstream \x1b]52;c;clipboard-owned\x07unavailable"), nil)
	if fallback.DialogTitle() != "API Error" || !strings.Contains(fallback.DialogMessage(), "HTTP 502 Bad Gateway") || !strings.Contains(fallback.DialogMessage(), "upstream unavailable") {
		t.Fatalf("generic API error fallback = title %q message %q", fallback.DialogTitle(), fallback.DialogMessage())
	}
	if strings.Contains(fallback.DialogMessage()+fallback.Details(), "clipboard-owned") {
		t.Fatalf("terminal-control payload reached API error presentation: %q", fallback.DialogMessage()+fallback.Details())
	}
}

func TestErrorDialogUsesRegistryScrollingAndRestoresPreviousMode(t *testing.T) {
	model := &Model{mode: modeConfirmation, shell: NewShell("secret-token")}
	lines := make([]string, 30)
	for index := range lines {
		lines[index] = fmt.Sprintf("line %02d", index)
	}
	requestErr := &APIError{
		OperationID: "deleteThing", Method: "DELETE", URL: "https://example.test/things/one",
		Status: 500, Kind: "Error", Reason: "Unable to delete thing", Code: "rh-trex-ai-1",
		Body: strings.Join(lines, "\n") + "\nsecret-token\nfinal marker",
	}
	model.presentRequestError("request:deleteThing", requestErr, false)
	if model.mode != modeErrorDialog || model.previousMode != modeConfirmation || model.errorDialog == nil {
		t.Fatalf("foreground API error did not open over source mode: mode=%v previous=%v dialog=%#v", model.mode, model.previousMode, model.errorDialog)
	}
	model.errorDialog.SetSize(24, 8)
	compact := ansi.Strip(model.errorDialog.Content(PlainTheme()))
	if !strings.Contains(compact, "Unable to delete thing") || !strings.Contains(compact, "[ Close ]") || !strings.Contains(compact, "Details") || strings.Contains(compact, "line 00") {
		t.Fatalf("compact error summary is incorrect: %q", compact)
	}
	_, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if model.mode != modeConfirmation || model.errorDialog != nil {
		t.Fatalf("default close did not restore source mode: mode=%v dialog=%#v", model.mode, model.errorDialog)
	}
	model.presentRequestError("request:deleteThing", requestErr, false)
	model.errorDialog.SetSize(24, 8)
	_, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyRight})
	_, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !model.errorDialog.Expanded() || strings.Contains(ansi.Strip(model.errorDialog.Content(PlainTheme())), "final marker") {
		t.Fatal("details did not open at the beginning of the full error")
	}
	_, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyEnd})
	if !strings.Contains(ansi.Strip(model.errorDialog.Content(PlainTheme())), "final marker") {
		t.Fatal("error dialog end binding did not make the complete error reachable")
	}
	_, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if model.mode != modeErrorDialog || model.errorDialog == nil || model.errorDialog.Expanded() {
		t.Fatalf("details back did not restore compact error: mode=%v dialog=%#v", model.mode, model.errorDialog)
	}
	_, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if model.mode != modeConfirmation || model.errorDialog != nil {
		t.Fatalf("error dialog dismissal did not restore source mode: mode=%v dialog=%#v", model.mode, model.errorDialog)
	}
}

func TestFormDialogFocusEnumValidationZeroAndDuplicateSubmit(t *testing.T) {
	operation := Operation{
		ID: "createItem", Method: "POST",
		Parameters: []Parameter{{Name: "dry_run", In: "query", Type: "boolean", Default: false}},
		RequestBody: &RequestBody{Required: true, ContentType: "application/json", Fields: []InputField{
			{Name: "count", Type: "integer", Required: true, Default: 0},
			{Name: "state", Type: "string", Required: true, Enum: []any{"new", "ready"}, Default: "new"},
		}},
	}
	keys := DefaultKeyRegistry()
	form := NewFormDialog(operation, map[string]any{}, keys)
	if len(form.fields) != 3 || form.fields[0].descriptor.Name != "count" || form.fields[1].descriptor.Name != "state" || form.fields[2].descriptor.Name != "dry_run" {
		t.Fatalf("deterministic fields = %#v", form.fields)
	}
	content := form.Content(PlainTheme())
	if !strings.Contains(content, "count") || !strings.Contains(content, "integer") || !strings.Contains(content, "required") ||
		!strings.Contains(content, "dry_run") || !strings.Contains(content, "boolean") || !strings.Contains(content, "optional") ||
		strings.Contains(content, "body field") || strings.Contains(content, "query parameter") ||
		!strings.Contains(content, "‹ new ›") {
		t.Fatalf("form content = %q", content)
	}
	inputColumns := make(map[int]bool)
	for _, line := range strings.Split(content, "\n") {
		if separator := strings.Index(line, ": "); separator >= 0 {
			inputColumns[ansi.StringWidth(line[:separator+2])] = true
		}
	}
	if len(inputColumns) != 1 {
		t.Fatalf("form inputs are not display-cell aligned: columns %v\n%s", inputColumns, content)
	}
	theme := DefaultTheme()
	if !theme.FieldTitleStyle.GetBold() || theme.FieldTitleStyle.GetForeground() == theme.Muted.GetForeground() {
		t.Fatal("field title is not emphasized with a distinct bright foreground")
	}
	_, _ = form.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_, _ = form.Update(tea.KeyMsg{Type: tea.KeyEnter})
	event, _ := form.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !event.Submitted || string(event.Request.Body) != `{"count":0,"state":"new"}` {
		t.Fatalf("zero/default submission = event %#v, body %s", event, event.Request.Body)
	}
	duplicate, _ := form.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if duplicate.Submitted {
		t.Fatal("in-flight form accepted duplicate submission")
	}
}

func TestFormInlineErrorAndFooterUseSharedSemanticStyles(t *testing.T) {
	keys := DefaultKeyRegistry()
	form := NewFormDialog(Operation{
		ID: "createItem",
		RequestBody: &RequestBody{Required: true, Fields: []InputField{
			{Name: "observed_at", Type: "string", Format: "date-time", Required: true},
			{Name: "名", Type: "integer", Required: false},
		}},
	}, nil, keys)
	form.fields[0].input.SetValue("invalid")
	event, _ := form.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !event.Invalid {
		t.Fatal("invalid date-time field was accepted")
	}
	theme := DefaultTheme()
	content := form.Content(theme)
	wantError := theme.FieldError("! must be an RFC 3339 date-time")
	if !strings.Contains(content, wantError) {
		t.Fatalf("inline validation does not use the danger style:\n%s", content)
	}

	footer := form.Footer(PlainTheme())
	cancel := "[esc] back/cancel"
	submit := "[enter] select/submit"
	if cancelAt, submitAt := strings.Index(footer, cancel), strings.Index(footer, submit); cancelAt < 0 || submitAt < 0 || cancelAt > submitAt || !strings.HasSuffix(footer, submit) {
		t.Fatalf("form footer does not end with cancel then submit: %q", footer)
	}
	submitHint, present := keys.Hint(KeySubmit)
	if !present || !strings.Contains(form.Footer(theme), theme.DialogAction(submitHint, true)) {
		t.Fatalf("form submit action is not primary-styled: %q", form.Footer(theme))
	}
}

func TestRequiredStructuredBodySubmitsEmptyObject(t *testing.T) {
	form := NewFormDialog(Operation{
		ID: "createItem",
		RequestBody: &RequestBody{Required: true, ContentType: "application/json", Fields: []InputField{
			{Name: "state", Type: "string", Enum: []any{"new", "ready"}},
		}},
	}, nil, DefaultKeyRegistry())
	if !strings.Contains(form.Content(PlainTheme()), "‹ unset ›") {
		t.Fatalf("optional enum was not initially unset: %q", form.Content(PlainTheme()))
	}
	event, _ := form.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !event.Submitted || string(event.Request.Body) != `{}` {
		t.Fatalf("required empty body = event %#v, body %q", event, event.Request.Body)
	}
}

func TestDestructiveConfirmationStartsOnCancelAndSubmitsOnce(t *testing.T) {
	keys := DefaultKeyRegistry()
	dialog := NewConfirmationDialog("Delete", Confirmation{Title: "Confirm delete", Message: "Delete it?", Destructive: true}, keys)
	content := dialog.Content(PlainTheme())
	if !strings.Contains(content, "[ Cancel ]") || !strings.Contains(content, "Delete") || strings.Contains(content, "DESTRUCTIVE") || dialog.Footer(PlainTheme()) != "" {
		t.Fatalf("compact safe confirmation = content %q, footer %q", content, dialog.Footer(PlainTheme()))
	}
	host := ModalHost{}
	host.Open(dialog)
	rendered := host.Render("background", 80, 20, PlainTheme())
	if !strings.Contains(rendered, "<Confirm delete>") || strings.Contains(rendered, "select/submit") || strings.Contains(rendered, "back/cancel") {
		t.Fatalf("confirmation overlay has redundant or missing chrome:\n%s", rendered)
	}
	confirmed, canceled := dialog.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if confirmed || !canceled {
		t.Fatalf("enter on safe focus = confirmed %v, canceled %v", confirmed, canceled)
	}
	dialog = NewConfirmationDialog("Delete", Confirmation{Title: "Confirm delete", Message: "Delete it?", Destructive: true}, keys)
	_, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if !strings.Contains(dialog.Content(PlainTheme()), "[ Cancel ]") {
		t.Fatalf("left arrow moved past Cancel: %q", dialog.Content(PlainTheme()))
	}
	_, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyRight})
	_, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyRight})
	if !strings.Contains(dialog.Content(PlainTheme()), "[ Delete ]") {
		t.Fatalf("right arrow did not hold Delete focus: %q", dialog.Content(PlainTheme()))
	}
	_, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	_, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyTab})
	confirmed, canceled = dialog.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !confirmed || canceled {
		t.Fatalf("explicit confirmation = confirmed %v, canceled %v", confirmed, canceled)
	}
	confirmed, _ = dialog.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if confirmed {
		t.Fatal("in-flight confirmation accepted duplicate submit")
	}
}

func TestResourceTableSortAndCommandHistoryAreShared(t *testing.T) {
	view := View{ID: "records", Kind: "collection", IdentityProperty: "id", DefaultSort: "name", Columns: []Column{{Property: "name", Label: "NAME"}, {Property: "id", Label: "ID"}}}
	component := ResourceTableComponent{}
	component.Reset(view, PlainTheme(), 50, 8, 0)
	component.SetRows(view, []map[string]any{{"id": "2", "name": "Beta"}, {"id": "1", "name": "Alpha"}}, "", "", 50, 8, 0)
	component.table.SetCursor(1)
	component.CycleSort(view)
	selected := component.Selected()
	if component.sortProperty != "id" || selected == nil || selected.Identity != "2" {
		t.Fatalf("sort cycle lost state: property %q, selected %#v", component.sortProperty, component.Selected())
	}
	component.ReverseSort()
	if component.rows[0].Identity != "2" || component.Selected() == nil || component.Selected().Identity != "2" {
		t.Fatalf("reverse sort lost order or selection: rows %#v, selected %#v", component.rows, component.Selected())
	}

	bar := NewCommandBar()
	bar.Remember("records")
	bar.Remember("accounts")
	bar.MoveHistory(-1)
	if bar.Value() != "accounts" {
		t.Fatalf("latest command history = %q", bar.Value())
	}
	bar.MoveHistory(-1)
	if bar.Value() != "records" {
		t.Fatalf("previous command history = %q", bar.Value())
	}
	bar.Begin(CommandResource, "a")
	bar.SetSuggestions([]string{"accounts"})
	if bar.CurrentSuggestion() != "accounts" {
		t.Fatalf("resource suggestion = %q", bar.CurrentSuggestion())
	}
	bar.Begin(CommandFilter, "a")
	if bar.CurrentSuggestion() != "" {
		t.Fatalf("filter inherited resource suggestion %q", bar.CurrentSuggestion())
	}
}

func TestNarrowSortedHeaderKeepsDirectionAsLeftPrefix(t *testing.T) {
	view := View{
		ID: "records", Kind: "collection", DefaultSort: "description",
		Columns: []Column{{Property: "description", Label: "VERY LONG DESCRIPTION", Type: "string"}},
	}
	component := ResourceTableComponent{}
	component.Reset(view, PlainTheme(), 8, 5, 0)
	component.SetRows(view, []map[string]any{{"description": "value"}}, "", "", 8, 5, 0)
	if title := component.table.Columns()[0].Title; title != "↑ VERY LONG DESCRIPTION" {
		t.Fatalf("ascending header title = %q", title)
	}
	if rendered := ansi.Strip(component.View()); !strings.Contains(rendered, "↑") || !strings.Contains(rendered, "…") {
		t.Fatalf("narrow ascending header hid sort prefix: %q", rendered)
	}

	component.ReverseSort()
	component.Configure(view, 8, 5, 0)
	if title := component.table.Columns()[0].Title; title != "↓ VERY LONG DESCRIPTION" {
		t.Fatalf("descending header title = %q", title)
	}
	if rendered := ansi.Strip(component.View()); !strings.Contains(rendered, "↓") || !strings.Contains(rendered, "…") {
		t.Fatalf("narrow descending header hid sort prefix: %q", rendered)
	}
}

func TestControlActionHotkeysUseMetadataGrammarAndBubbleTeaDispatch(t *testing.T) {
	registry := DefaultKeyRegistry()
	if !registry.Reserved("ctrl-c") {
		t.Fatal("metadata-form global control key was not reserved")
	}
	if !registry.Reserved("r") {
		t.Fatal("raw-resource key was not reserved")
	}
	operation := Operation{Presentation: ActionPresentation{Hotkey: "ctrl-z"}}
	if !registry.ActionMatches(tea.KeyMsg{Type: tea.KeyCtrlZ}, operation) {
		t.Fatal("metadata-form control key did not match Bubble Tea key event")
	}
}
