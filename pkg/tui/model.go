package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type Row struct {
	Raw      map[string]any
	Identity string
	Cells    []string
}

type actionCandidate struct {
	Operation Operation
	Values    map[string]any
}

type Frame struct {
	ID               uint64
	Catalog          bool
	CatalogSelection string
	EdgeID           string
	SourceViewID     string
	SelectedIdentity string
	ColumnOffset     int
	Bindings         map[string]any
	TargetViewID     string
	Label            string
	Selected         *Row
	RequestValues    map[string]any
	RequestBody      any
	RequestURL       string
	RequestMethod    string
	ResponseHeaders  http.Header
	ResponseBody     any
	ResponseStatus   int
	LastSuccess      time.Time
	RefreshIdentity  string
	InFlight         bool
	Refreshing       bool
	Stale            bool
	Forbidden        bool
	LoadFailed       bool
}

type mode int

const (
	modeBrowse mode = iota
	modeCatalog
	modeFilter
	modeSwitch
	modeRelationships
	modeActions
	modeActionInput
	modeConfirmation
	modeDetail
	modeRaw
	modeErrorDialog
	modeHelp
	modeAlertDetails
)

const (
	maxVisibleStreamEvents = 500
	maxVisibleStreamBytes  = 1 << 20
	maxActionInputBytes    = 64 << 10
)

type Model struct {
	descriptor Descriptor
	client     *Client
	frames     []Frame
	ResourceTableComponent
	DetailStreamComponent
	CommandBar
	chooser          table.Model
	mode             mode
	previousMode     mode
	rawReturnMode    mode
	width            int
	height           int
	loading          bool
	chosenEdges      []Edge
	chosenOperations []Operation
	chosenValues     []map[string]any
	actionOperation  Operation
	actionRequest    RequestInput
	form             *FormDialog
	confirmation     *ConfirmationDialog
	errorDialog      *ErrorDialog
	actionInFlight   bool
	shell            Shell
	refreshInterval  time.Duration
	nextRefresh      time.Time
	nextFrameID      uint64
	now              func() time.Time
}

type operationResultMsg struct {
	viewID      string
	frameID     uint64
	operationID string
	input       RequestInput
	result      Result
	err         error
	background  bool
}

type presentationPulseMsg struct{ now time.Time }

type streamOpenedMsg struct {
	viewID  string
	frameID uint64
	events  <-chan streamEvent
	err     error
}

type streamEventMsg struct {
	viewID  string
	frameID uint64
	events  <-chan streamEvent
	event   streamEvent
}

type streamEvent struct {
	text string
	err  error
	done bool
}

func NewModel(descriptor Descriptor, config ClientConfig) (*Model, error) {
	if config.RefreshInterval < 0 {
		return nil, fmt.Errorf("refresh interval must be non-negative")
	}
	client, err := NewClient(descriptor, config)
	if err != nil {
		return nil, err
	}
	if !hasCollectionView(descriptor) {
		return nil, fmt.Errorf("descriptor has no collection view")
	}
	detail := viewport.New(80, 20)
	model := &Model{
		descriptor: descriptor, client: client, CommandBar: NewCommandBar(), DetailStreamComponent: DetailStreamComponent{detail: detail, autoscroll: true},
		width: 100, height: 30,
		frames:          []Frame{{ID: 1, Catalog: true, Label: resourceCatalogLabel, Bindings: map[string]any{}}},
		mode:            modeCatalog,
		shell:           NewShell(config.Token),
		refreshInterval: config.RefreshInterval, nextFrameID: 1, now: time.Now,
	}
	if config.RefreshInterval > 0 {
		model.nextRefresh = model.now().Add(config.RefreshInterval)
	}
	model.rebuildCatalog()
	return model, nil
}

func (model *Model) Init() tea.Cmd {
	return presentationPulse()
}

func (model *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := message.(type) {
	case tea.WindowSizeMsg:
		model.width, model.height = typed.Width, typed.Height
		model.resize()
		return model, nil
	case operationResultMsg:
		return model.handleResult(typed)
	case presentationPulseMsg:
		model.shell.Alerts.Expire()
		model.updateStaleness(typed.now)
		var refresh tea.Cmd
		if model.refreshInterval > 0 && !typed.now.Before(model.nextRefresh) {
			model.nextRefresh = typed.now.Add(model.refreshInterval)
			refresh = model.refreshCurrent()
		}
		return model, tea.Batch(refresh, presentationPulse())
	case streamOpenedMsg:
		if model.currentView() == nil || len(model.frames) == 0 || typed.viewID != model.currentView().ID || typed.frameID != model.frames[len(model.frames)-1].ID {
			model.clearFrameRequest(typed.frameID)
			return model, nil
		}
		model.loading = false
		model.frames[len(model.frames)-1].InFlight = false
		if typed.err != nil {
			failedAction := model.actionInFlight
			if failedAction {
				model.restoreFailedActionWorkflow()
			}
			frame := &model.frames[len(model.frames)-1]
			if !failedAction {
				frame.LoadFailed = true
				frame.Stale = !frame.LastSuccess.IsZero()
			}
			model.presentRequestError("stream", typed.err, false)
			return model, nil
		}
		if model.actionInFlight {
			model.actionInFlight = false
			model.form = nil
			model.confirmation = nil
		}
		model.streamEvents = typed.events
		model.streamLines = nil
		model.streamBytes = 0
		model.connected = true
		model.autoscroll = true
		model.detailValue = nil
		model.detail.SetContent("")
		model.mode = modeDetail
		model.alertSuccess("stream-state", "Stream connected")
		frame := &model.frames[len(model.frames)-1]
		frame.LastSuccess = model.currentTime()
		frame.Stale = false
		return model, waitStreamEvent(typed.viewID, typed.frameID, typed.events)
	case streamEventMsg:
		if typed.events != model.streamEvents || model.currentView() == nil || len(model.frames) == 0 || typed.viewID != model.currentView().ID || typed.frameID != model.frames[len(model.frames)-1].ID {
			return model, nil
		}
		if typed.event.err != nil {
			frame := &model.frames[len(model.frames)-1]
			frame.LoadFailed = true
			frame.Stale = !frame.LastSuccess.IsZero()
			model.presentRequestError("stream", typed.event.err, false)
			model.streamEvents = nil
			model.streamCancel = nil
			model.connected = false
			return model, nil
		}
		if typed.event.done {
			model.alertInfo("stream-state", "Stream closed")
			model.streamEvents = nil
			model.streamCancel = nil
			model.connected = false
			return model, nil
		}
		model.appendStreamEvent(typed.event.text)
		return model, waitStreamEvent(typed.viewID, typed.frameID, typed.events)
	case tea.KeyMsg:
		return model.handleKey(typed)
	}
	if model.mode == modeCatalog || model.mode == modeBrowse {
		var command tea.Cmd
		model.table, command = model.table.Update(message)
		return model, command
	}
	if model.mode == modeDetail || model.mode == modeRaw {
		var command tea.Cmd
		model.detail, command = model.detail.Update(message)
		return model, command
	}
	if model.mode == modeErrorDialog && model.errorDialog != nil {
		return model.handleErrorDialogMessage(message)
	}
	return model, nil
}

func (model *Model) View() string {
	model.ensurePresentation()
	view := model.currentView()
	if view == nil {
		return model.shell.Render(ShellView{Page: SemanticPage{PageTitle: "Unavailable", PageState: PageFatal, PageContent: "No view"}}, model.width, model.height)
	}
	page := model.semanticPage(*view)
	command := model.commandPrompt()
	switch model.mode {
	case modeFilter, modeSwitch:
	case modeRelationships, modeActions:
		title := "Relationships"
		if model.mode == modeActions {
			title = "Actions"
		}
		model.shell.Modal.Open(StaticDialog{DialogKind: DialogChoice, DialogTitle: title, DialogContent: model.chooser.View(), DialogFooter: model.shell.Keys.Hints(KeySubmit, KeyCancel)})
	case modeActionInput:
		if model.form != nil {
			model.shell.Modal.Open(model.form)
		}
	case modeConfirmation:
		if model.confirmation != nil {
			model.shell.Modal.Open(model.confirmation)
		}
	case modeErrorDialog:
		if model.errorDialog != nil {
			model.shell.Modal.Open(model.errorDialog)
		}
	case modeHelp:
		model.shell.Modal.Open(StaticDialog{DialogKind: DialogHelp, DialogTitle: "Help", DialogContent: model.helpContent(*view), DialogFooter: model.shell.Keys.Hints(KeyCancel)})
	case modeAlertDetails:
		if alert, present := model.shell.Alerts.Active(); present {
			details := alert.Details
			if details == "" {
				details = alert.Summary
			}
			model.shell.Modal.Open(StaticDialog{DialogKind: DialogHelp, DialogTitle: alertPrefix(alert.Severity) + " details", DialogContent: details, DialogFooter: model.shell.Keys.Hints(KeyCancel, KeyDismissAlert)})
		}
	default:
		model.shell.Modal.Close()
	}
	presentation := model.shellView(*view, page, command)
	return model.shell.Render(presentation, model.width, model.height)
}

func (model *Model) commandPrompt() *CommandPromptView {
	if model.mode != modeFilter && model.mode != modeSwitch {
		return nil
	}
	view := model.CommandBar.View(model.shell.Theme, model.width)
	return &view
}

func (model *Model) shellView(view View, page Page, command *CommandPromptView) ShellView {
	return ShellView{
		Header: HeaderModel{
			Service: model.descriptor.Title, Origin: model.serverOrigin(),
			Authenticated: model.authenticated(), Scope: model.scope(), Refreshing: model.currentRefreshing(),
			LastSuccess: model.currentLastSuccess(), Now: model.currentTime(),
		},
		Page: page, Command: command, Breadcrumb: model.breadcrumb(), HintIDs: model.applicableKeys(),
	}
}

func (model *Model) semanticPage(view View) Page {
	state := PageReady
	frame := &model.frames[len(model.frames)-1]
	if frame.Forbidden {
		state = PageForbidden
	} else if frame.LoadFailed && len(model.rows) == 0 && frame.Selected == nil {
		state = PageFatal
	} else if frame.Stale {
		state = PageStale
	} else if model.loading && len(model.rows) == 0 && frame.Selected == nil {
		state = PageLoading
	}
	var actions []LocalAction
	if model.mode == modeBrowse {
		actions = model.localActions(view)
	}
	if model.mode == modeRaw {
		return DetailPage{SemanticPage{PageTitle: "Raw " + view.Label, PageScope: model.scope(), PageState: state, PageContent: model.shell.Theme.DetailBody(model.detail.View())}}
	}
	if model.mode == modeDetail {
		return DetailPage{SemanticPage{PageTitle: view.Label, PageScope: model.scope(), PageState: state, PageContent: model.shell.Theme.DetailBody(model.detail.View()), PageActions: actions}}
	}
	if view.Kind == "stream" {
		return StreamPage{SemanticPage{PageTitle: view.Label, PageScope: model.scope(), PageState: state, PageContent: model.StreamContent(), PageActions: actions}}
	}
	count := len(model.visible)
	if state == PageReady && !model.loading && count == 0 {
		state = PageEmpty
	}
	if model.onCatalog() {
		return ResourceTablePage{SemanticPage{PageTitle: view.Label, PageFilter: model.filter, PageSimpleTitle: true, PageState: state, PageContent: model.tableView(), PageActions: actions}}
	}
	return ResourceTablePage{SemanticPage{PageTitle: view.Label, PageScope: model.tableScope(), PageCount: &count, PageFilter: model.filter, PageState: state, PageContent: model.tableView(), PageActions: actions}}
}

func (model *Model) applicableKeys() []BindingID {
	return model.applicableKeysForMode(model.mode)
}

func (model *Model) applicableKeysForMode(activeMode mode) []BindingID {
	var keys []BindingID
	switch activeMode {
	case modeHelp:
		keys = []BindingID{KeyCancel, KeyHelp, KeyForceQuit}
	case modeAlertDetails:
		keys = []BindingID{KeyCancel, KeyHelp, KeyDismissAlert, KeyForceQuit}
	case modeErrorDialog:
		if model.errorDialog != nil && model.errorDialog.Expanded() {
			keys = []BindingID{KeyScrollUp, KeyScrollDown, KeyPageUp, KeyPageDown, KeyScrollHome, KeyScrollEnd, KeyCancel, KeyForceQuit}
		} else {
			keys = []BindingID{KeyChoicePrevious, KeyChoiceNext, KeyNextFocus, KeyPreviousFocus, KeySubmit, KeyCancel, KeyForceQuit}
		}
	case modeFilter, modeSwitch:
		keys = []BindingID{KeySubmit, KeyCancel, KeyHelp, KeyForceQuit}
		if activeMode == modeSwitch {
			keys = append(keys, KeySuggestionNext, KeySuggestionPrev, KeyAcceptSuggestion)
		} else {
			keys = append(keys, KeyHistoryPrevious, KeyHistoryNext)
		}
	case modeRelationships, modeActions:
		keys = []BindingID{KeySubmit, KeyCancel, KeyHelp, KeyForceQuit}
	case modeActionInput:
		keys = []BindingID{KeyNextFocus, KeyPreviousFocus, KeyChoicePrevious, KeyChoiceNext, KeySubmit, KeyCancel, KeyHelp, KeyForceQuit}
	case modeConfirmation:
		keys = []BindingID{KeyNextFocus, KeySubmit, KeyCancel, KeyHelp, KeyForceQuit}
	case modeDetail:
		keys = []BindingID{KeyNavigate, KeyCancel, KeyHelp, KeyQuit}
	case modeRaw:
		keys = []BindingID{KeyCancel, KeyHelp, KeyQuit}
	case modeCatalog:
		keys = []BindingID{KeyCommand, KeyFilter, KeyNavigate, KeySortNext, KeySortDirection, KeyHelp, KeyQuit}
	default:
		keys = []BindingID{KeyCommand, KeyFilter, KeyNavigate, KeyDetail, KeyActions, KeySortNext, KeySortDirection, KeyCancel, KeyHelp, KeyQuit}
	}
	if activeMode != modeHelp && activeMode != modeAlertDetails && activeMode != modeErrorDialog {
		if _, present := model.shell.Alerts.Active(); present {
			keys = append(keys, KeyAlertDetails, KeyDismissAlert)
		}
	}
	if (activeMode == modeCatalog || activeMode == modeBrowse) && model.leftOverflow > 0 {
		keys = append(keys, KeyColumnsLeft)
	}
	if (activeMode == modeCatalog || activeMode == modeBrowse) && model.rightOverflow > 0 {
		keys = append(keys, KeyColumnsRight)
	}
	if view := model.currentView(); activeMode == modeDetail && view != nil && view.Kind == "stream" {
		keys = append(keys, KeyToggleAutoscroll)
	}
	if model.rawResourceAvailable(activeMode) {
		keys = append(keys, KeyRaw)
	}
	return keys
}

func (model *Model) rawResourceAvailable(activeMode mode) bool {
	if activeMode != modeBrowse && activeMode != modeDetail {
		return false
	}
	view := model.currentView()
	return view != nil && !model.onCatalog() && view.Kind != "stream" && model.selectedRow() != nil
}

func (model *Model) helpContent(view View) string {
	helpMode := model.mode
	if helpMode == modeHelp {
		helpMode = model.previousMode
	}
	var actions []LocalAction
	if helpMode == modeBrowse {
		actions = model.localActions(view)
	}
	return model.shell.Keys.ShortcutHelp(model.applicableKeysForMode(helpMode), actions)
}

func (model *Model) localActions(view View) []LocalAction {
	var result []LocalAction
	var selected *Row
	if current := model.currentView(); current != nil && current.ID == view.ID {
		selected = model.selectedRow()
	}
	for _, candidate := range model.actionCandidates(view, selected) {
		operation := candidate.Operation
		if operation.Presentation.Hotkey == "" {
			continue
		}
		result = append(result, LocalAction{Label: actionLabel(operation), Hotkey: operation.Presentation.Hotkey})
	}
	return result
}

func (model *Model) scope() string {
	if len(model.frames) == 0 {
		return ""
	}
	bindings := model.frames[len(model.frames)-1].Bindings
	keys := sortedAnyKeys(bindings)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+SanitizeCell(scalarString(bindings[key])))
	}
	return strings.Join(parts, ", ")
}

func (model *Model) tableScope() string {
	parts := make([]string, 0, 2)
	if scope := model.scope(); scope != "" {
		parts = append(parts, scope)
	}
	if model.filter != "" {
		parts = append(parts, `filter="`+SanitizeCell(model.filter)+`"`)
	}
	return strings.Join(parts, " · ")
}

func (model *Model) tableView() string {
	return model.ResourceTableComponent.View()
}

func (model *Model) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	model.ensurePresentation()
	if model.shell.Keys.Matches(key, KeyForceQuit) || (model.shell.Keys.Matches(key, KeyQuit) && (model.mode == modeCatalog || model.mode == modeBrowse || model.mode == modeDetail || model.mode == modeRaw)) {
		model.cancelStream()
		return model, tea.Quit
	}
	if model.mode == modeErrorDialog {
		return model.handleErrorDialogMessage(key)
	}
	if model.mode == modeHelp || model.mode == modeAlertDetails {
		if model.mode == modeAlertDetails && model.shell.Keys.Matches(key, KeyDismissAlert) {
			model.shell.Alerts.Dismiss()
			model.mode = model.previousMode
			model.shell.Modal.Close()
			return model, nil
		}
		if model.shell.Keys.Matches(key, KeyCancel) || model.shell.Keys.Matches(key, KeyHelp) {
			model.mode = model.previousMode
			model.shell.Modal.Close()
		}
		return model, nil
	}
	if model.shell.Keys.Matches(key, KeyHelp) {
		model.previousMode = model.mode
		model.mode = modeHelp
		return model, nil
	}
	if model.shell.Keys.Matches(key, KeyAlertDetails) {
		if _, present := model.shell.Alerts.Active(); present {
			model.previousMode = model.mode
			model.mode = modeAlertDetails
		}
		return model, nil
	}
	if model.shell.Keys.Matches(key, KeyDismissAlert) {
		model.shell.Alerts.Dismiss()
		return model, nil
	}
	if model.mode == modeFilter || model.mode == modeSwitch {
		return model.handleInputKey(key)
	}
	if model.mode == modeActionInput {
		return model.handleFormKey(key)
	}
	if model.mode == modeConfirmation {
		return model.handleConfirmationKey(key)
	}
	if model.mode == modeRelationships || model.mode == modeActions {
		return model.handleChooserKey(key)
	}
	if model.mode == modeRaw {
		if model.shell.Keys.Matches(key, KeyCancel) {
			model.closeRawResource()
			return model, nil
		}
		var command tea.Cmd
		model.detail, command = model.detail.Update(key)
		return model, command
	}
	if model.mode == modeDetail {
		if model.shell.Keys.Matches(key, KeyRaw) {
			return model.openRawResource()
		}
		if model.shell.Keys.Matches(key, KeyCancel) {
			view := model.currentView()
			wasStreaming := model.streamEvents != nil || model.streamCancel != nil
			model.cancelStream()
			if len(model.frames) > 1 && view != nil && (view.Kind == "stream" || (!wasStreaming && view.Kind == "item")) {
				return model.popFrame()
			}
			model.mode = modeBrowse
			return model, nil
		}
		if model.shell.Keys.Matches(key, KeyNavigate) {
			return model.followRelationships()
		}
		if view := model.currentView(); view != nil && view.Kind == "stream" && model.shell.Keys.Matches(key, KeyToggleAutoscroll) {
			model.autoscroll = !model.autoscroll
			if model.autoscroll {
				model.detail.GotoBottom()
			}
			return model, nil
		}
		var command tea.Cmd
		model.detail, command = model.detail.Update(key)
		return model, command
	}
	if direction := columnScrollDirection(key); direction != 0 {
		model.scrollColumns(direction)
		return model, nil
	}
	switch {
	case model.shell.Keys.Matches(key, KeyFilter):
		model.previousMode = model.mode
		model.mode = modeFilter
		return model, model.CommandBar.Begin(CommandFilter, model.filter)
	case model.shell.Keys.Matches(key, KeyCommand):
		model.previousMode = model.mode
		model.mode = modeSwitch
		command := model.CommandBar.Begin(CommandResource, "")
		model.CommandBar.SetSuggestions(model.resourceCommandCandidates())
		return model, command
	case model.shell.Keys.Matches(key, KeyCancel):
		if model.filter != "" {
			model.filter = ""
			model.applyFilter()
			return model, nil
		}
		return model.popFrame()
	case model.shell.Keys.Matches(key, KeyNavigate):
		return model.followRelationships()
	case model.shell.Keys.Matches(key, KeyDetail):
		if model.onCatalog() {
			return model, nil
		}
		row := model.selectedRow()
		if row == nil {
			model.alertWarning("selection", "No selected item")
			return model, nil
		}
		model.ShowDetail(row.Raw, model.shell.Theme)
		model.mode = modeDetail
		return model, nil
	case model.shell.Keys.Matches(key, KeyRaw):
		return model.openRawResource()
	case model.shell.Keys.Matches(key, KeyActions):
		if model.onCatalog() {
			return model, nil
		}
		return model.openActions()
	case model.shell.Keys.Matches(key, KeySortNext):
		if view := model.currentView(); view != nil {
			model.ResourceTableComponent.CycleSort(*view)
			model.configureTableColumns(*view)
			model.applyFilter()
		}
		return model, nil
	case model.shell.Keys.Matches(key, KeySortDirection):
		model.ResourceTableComponent.ReverseSort()
		if view := model.currentView(); view != nil {
			model.configureTableColumns(*view)
			model.applyFilter()
		}
		return model, nil
	}
	for _, candidate := range model.actionCandidatesForCurrentView() {
		if model.shell.Keys.ActionMatches(key, candidate.Operation) {
			return model.beginActionWithValues(candidate.Operation, candidate.Values)
		}
	}
	var command tea.Cmd
	model.table, command = model.table.Update(key)
	return model, command
}

func (model *Model) openRawResource() (tea.Model, tea.Cmd) {
	if model.onCatalog() {
		return model, nil
	}
	row := model.selectedRow()
	if row == nil {
		model.alertWarning("selection", "No selected item")
		return model, nil
	}
	raw, err := renderRaw(row.Raw)
	if err != nil {
		model.alertError("raw", err.Error())
		return model, nil
	}
	model.rawReturnMode = model.mode
	model.ShowRaw(raw, model.shell.Theme)
	model.mode = modeRaw
	return model, nil
}

func (model *Model) closeRawResource() {
	returnMode := model.rawReturnMode
	if returnMode == modeDetail {
		if row := model.selectedRow(); row != nil {
			model.ShowDetail(row.Raw, model.shell.Theme)
		}
	} else {
		returnMode = modeBrowse
	}
	model.mode = returnMode
}

func (model *Model) handleInputKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if model.shell.Keys.Matches(key, KeyCancel) {
		model.mode = model.commandReturnMode()
		model.CommandBar.Close()
		return model, nil
	}
	if model.mode == modeFilter && model.shell.Keys.Matches(key, KeyHistoryPrevious) {
		model.CommandBar.MoveHistory(-1)
		return model, nil
	}
	if model.mode == modeFilter && model.shell.Keys.Matches(key, KeyHistoryNext) {
		model.CommandBar.MoveHistory(1)
		return model, nil
	}
	if model.mode == modeSwitch && (model.shell.Keys.Matches(key, KeySuggestionNext) || model.shell.Keys.Matches(key, KeySuggestionPrev)) {
		return model, model.CommandBar.Update(key)
	}
	if model.mode == modeSwitch && model.shell.Keys.Matches(key, KeyAcceptSuggestion) {
		if model.CommandBar.AcceptSuggestion() {
			return model, nil
		}
		return model, model.CommandBar.Update(key)
	}
	if model.shell.Keys.Matches(key, KeySubmit) {
		value := strings.TrimSpace(model.CommandBar.Value())
		model.CommandBar.Remember(value)
		model.CommandBar.Close()
		if model.mode == modeFilter {
			model.filter = value
			model.applyFilter()
			model.mode = model.commandReturnMode()
			return model, nil
		}
		view := model.addressableView(value)
		if view == nil {
			model.alertError("command", "Unknown or unavailable resource: "+value)
			model.mode = model.commandReturnMode()
			return model, nil
		}
		bindings := availableBindings(model.frames)
		return model, model.openResourceView(view, bindings, true)
	}
	command := model.CommandBar.Update(key)
	if model.mode == modeFilter {
		model.filter = model.CommandBar.Value()
		model.applyFilter()
	}
	return model, command
}

func (model *Model) handleChooserKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if model.shell.Keys.Matches(key, KeyCancel) {
		model.mode = modeBrowse
		model.shell.Modal.Close()
		return model, nil
	}
	if model.shell.Keys.Matches(key, KeySubmit) {
		index := model.chooser.Cursor()
		if model.mode == modeRelationships && index < len(model.chosenEdges) {
			edge := model.chosenEdges[index]
			model.mode = modeBrowse
			return model.pushEdge(edge)
		}
		if model.mode == modeActions && index < len(model.chosenOperations) {
			operation := model.chosenOperations[index]
			if index < len(model.chosenValues) {
				return model.beginActionWithValues(operation, model.chosenValues[index])
			}
			return model.beginAction(operation)
		}
	}
	var command tea.Cmd
	model.chooser, command = model.chooser.Update(key)
	return model, command
}

func (model *Model) followRelationships() (tea.Model, tea.Cmd) {
	if model.onCatalog() {
		return model.openCatalogSelection()
	}
	view := model.currentView()
	if view == nil {
		return model, nil
	}
	edges := model.descriptor.Outgoing(view.ID)
	if len(edges) == 0 {
		row := model.selectedRow()
		if row != nil {
			model.ShowDetail(row.Raw, model.shell.Theme)
			model.mode = modeDetail
		}
		return model, nil
	}
	if len(edges) == 1 {
		return model.pushEdge(edges[0])
	}
	model.chosenEdges = edges
	rows := make([]table.Row, 0, len(edges))
	for _, edge := range edges {
		target := model.descriptor.View(edge.TargetViewID)
		label := edge.Name
		if target != nil {
			label = target.Label
		}
		rows = append(rows, table.Row{SanitizeCell(label), SanitizeCell(edge.Provenance)})
	}
	model.chooser = newChooser([]table.Column{{Title: "RELATIONSHIP", Width: 40}, {Title: "SOURCE", Width: 20}}, rows)
	model.mode = modeRelationships
	return model, nil
}

func (model *Model) pushEdge(edge Edge) (tea.Model, tea.Cmd) {
	row := model.selectedRow()
	if row == nil {
		model.alertWarning("selection", "Relationship requires a selected item")
		return model, nil
	}
	bindings, err := evaluateBindings(edge, model.frames[len(model.frames)-1], *row)
	if err != nil {
		model.alertError("relationship", err.Error())
		return model, nil
	}
	target := model.descriptor.View(edge.TargetViewID)
	if target == nil {
		model.alertError("relationship", "Relationship target is unavailable")
		return model, nil
	}
	frame := Frame{
		ID:     model.newFrameID(),
		EdgeID: edge.ID, SourceViewID: edge.SourceViewID, SelectedIdentity: row.Identity,
		Bindings: bindings, TargetViewID: target.ID, Label: target.Label, Selected: row,
	}
	model.frames = append(model.frames, frame)
	model.filter = ""
	model.mode = modeBrowse
	model.rebuildTable(*target)
	return model, model.loadCurrent()
}

func (model *Model) popFrame() (tea.Model, tea.Cmd) {
	if len(model.frames) <= 1 {
		return model, nil
	}
	model.cancelStream()
	model.restoreIdentity = model.frames[len(model.frames)-1].SelectedIdentity
	model.frames = model.frames[:len(model.frames)-1]
	view := model.currentView()
	if model.onCatalog() {
		model.filter = ""
		model.mode = modeCatalog
		model.restoreIdentity = model.frames[len(model.frames)-1].CatalogSelection
		model.rebuildCatalog()
		return model, nil
	}
	model.mode = modeBrowse
	if view != nil {
		model.rebuildTable(*view)
	}
	return model, model.loadCurrent()
}

func (model *Model) openActions() (tea.Model, tea.Cmd) {
	view := model.currentView()
	if view == nil {
		return model, nil
	}
	candidates := model.actionCandidatesForCurrentView()
	if len(candidates) == 0 {
		model.alertInfo("actions", "No documented actions for this view")
		return model, nil
	}
	model.chosenOperations = make([]Operation, 0, len(candidates))
	model.chosenValues = make([]map[string]any, 0, len(candidates))
	rows := make([]table.Row, 0, len(candidates))
	for _, candidate := range candidates {
		operation, values := candidate.Operation, candidate.Values
		model.chosenOperations = append(model.chosenOperations, operation)
		model.chosenValues = append(model.chosenValues, cloneBindings(values))
		label := actionLabel(operation)
		state := operation.Method
		if count := actionInputCount(operation, values); count > 0 {
			state += fmt.Sprintf(" · %d input(s)", count)
		}
		rows = append(rows, table.Row{SanitizeCell(label), SanitizeCell(state)})
	}
	model.chooser = newChooser([]table.Column{{Title: "ACTION", Width: 38}, {Title: "METHOD / INPUTS", Width: 46}}, rows)
	model.mode = modeActions
	return model, nil
}

func (model *Model) beginAction(operation Operation) (tea.Model, tea.Cmd) {
	for _, candidate := range model.actionCandidatesForCurrentView() {
		if candidate.Operation.ID == operation.ID {
			return model.beginActionWithValues(operation, candidate.Values)
		}
	}
	values := map[string]any{}
	if len(model.frames) > 0 {
		values = model.frames[len(model.frames)-1].Bindings
	}
	return model.beginActionWithValues(operation, values)
}

func (model *Model) beginActionWithValues(operation Operation, initialValues map[string]any) (tea.Model, tea.Cmd) {
	if len(model.frames) > 0 && model.frames[len(model.frames)-1].InFlight {
		model.alertWarning("action", "Wait for the current request to finish")
		return model, nil
	}
	values := cloneBindings(initialValues)
	for _, parameter := range operation.Parameters {
		if parameter.In != "path" {
			continue
		}
		if value, present := values[parameter.Name]; present {
			values[ParameterValueKey(parameter.In, parameter.Name)] = value
		}
	}
	model.actionOperation = operation
	model.actionRequest = RequestInput{Values: values}
	model.actionInFlight = false
	model.form = NewFormDialog(operation, values, model.shell.Keys)
	if len(model.form.fields) == 0 {
		return model.prepareActionSubmission(model.actionRequest)
	}
	model.mode = modeActionInput
	model.shell.Alerts.Clear("form-validation")
	return model, textinput.Blink
}

func (model *Model) handleFormKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if model.form == nil {
		model.mode = modeBrowse
		return model, nil
	}
	event, command := model.form.Update(message)
	if event.Canceled {
		model.form = nil
		model.mode = modeBrowse
		model.shell.Modal.Close()
		model.alertInfo("action", "Action canceled")
		return model, nil
	}
	if event.Invalid {
		model.alertError("form-validation", "Correct the highlighted fields before submitting")
		return model, command
	}
	if !event.Submitted {
		return model, command
	}
	model.shell.Alerts.Clear("form-validation")
	model.actionRequest = event.Request
	return model.prepareActionSubmission(event.Request)
}

func (model *Model) prepareActionSubmission(request RequestInput) (tea.Model, tea.Cmd) {
	if confirmation := model.actionOperation.Presentation.Confirmation; confirmation != nil {
		model.confirmation = NewConfirmationDialog(actionLabel(model.actionOperation), *confirmation, model.shell.Keys)
		model.mode = modeConfirmation
		return model, nil
	}
	model.actionInFlight = true
	if model.form != nil && len(model.form.fields) > 0 {
		model.mode = modeActionInput
	} else {
		model.mode = modeBrowse
		model.shell.Modal.Close()
	}
	return model, model.executeAction(model.actionOperation, request)
}

func (model *Model) handleConfirmationKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if model.confirmation == nil {
		model.mode = modeBrowse
		return model, nil
	}
	confirmed, canceled := model.confirmation.Update(message)
	if canceled {
		model.confirmation = nil
		model.form = nil
		model.mode = modeBrowse
		model.shell.Modal.Close()
		model.alertInfo("action", "Action canceled")
		return model, nil
	}
	if !confirmed || model.actionInFlight {
		return model, nil
	}
	model.actionInFlight = true
	return model, model.executeAction(model.actionOperation, model.actionRequest)
}

func parseParameterInput(parameter Parameter, value string) (any, error) {
	if parameter.Type != "array" && parameter.Type != "object" {
		if err := validateParameter(parameter, value); err != nil {
			return nil, fmt.Errorf("%s parameter %s: %w", parameter.In, parameter.Name, err)
		}
		return value, nil
	}
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	var parsed any
	if err := decoder.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("%s parameter %s must be valid JSON: %w", parameter.In, parameter.Name, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("%s parameter %s must contain one JSON value: %w", parameter.In, parameter.Name, err)
	}
	if err := validateParameter(parameter, parsed); err != nil {
		return nil, fmt.Errorf("%s parameter %s: %w", parameter.In, parameter.Name, err)
	}
	return parsed, nil
}

func (model *Model) executeAction(operation Operation, input RequestInput) tea.Cmd {
	if containsString(operation.Capabilities, "stream") {
		return model.openStream(operation, input)
	}
	return model.execute(operation, input)
}

func (model *Model) loadCurrent() tea.Cmd {
	return model.loadCurrentWithPurpose(false)
}

func (model *Model) loadCurrentWithPurpose(background bool) tea.Cmd {
	view := model.currentView()
	if view == nil || model.onCatalog() {
		return nil
	}
	if len(model.frames) > 0 && model.frames[len(model.frames)-1].InFlight {
		return nil
	}
	if view.ListOperationID != "" {
		operation := model.descriptor.Operation(view.ListOperationID)
		if operation != nil {
			return model.executeWithPurpose(*operation, RequestInput{Values: cloneBindings(model.frames[len(model.frames)-1].Bindings)}, background)
		}
	}
	if view.GetOperationID != "" {
		operation := model.descriptor.Operation(view.GetOperationID)
		if operation != nil {
			return model.executeWithPurpose(*operation, RequestInput{Values: cloneBindings(model.frames[len(model.frames)-1].Bindings)}, background)
		}
	}
	if len(view.StreamOperationIDs) > 0 {
		operation := model.descriptor.Operation(view.StreamOperationIDs[0])
		if operation != nil {
			return model.openStream(*operation, RequestInput{Values: cloneBindings(model.frames[len(model.frames)-1].Bindings)})
		}
	}
	model.alertError("read", "View has no executable read operation")
	return nil
}

func (model *Model) refreshCurrent() tea.Cmd {
	view := model.currentView()
	if view == nil || len(model.frames) == 0 || model.onCatalog() || view.Kind == "stream" || len(view.StreamOperationIDs) > 0 && view.ListOperationID == "" && view.GetOperationID == "" {
		return nil
	}
	frame := &model.frames[len(model.frames)-1]
	if frame.InFlight {
		return nil
	}
	if selected := model.selectedRow(); selected != nil {
		frame.RefreshIdentity = selected.Identity
	}
	return model.loadCurrentWithPurpose(true)
}

func (model *Model) execute(operation Operation, input RequestInput) tea.Cmd {
	return model.executeWithPurpose(operation, input, false)
}

func (model *Model) executeWithPurpose(operation Operation, input RequestInput, background bool) tea.Cmd {
	if len(model.frames) == 0 {
		return nil
	}
	frame := &model.frames[len(model.frames)-1]
	frame.ID = model.ensureFrameID(frame.ID)
	frame.InFlight = true
	if background {
		frame.Refreshing = true
	} else if containsString(operation.Capabilities, "list") || containsString(operation.Capabilities, "get") {
		model.loading = true
	}
	viewID, frameID := frame.TargetViewID, frame.ID
	return func() tea.Msg {
		result, err := model.client.Execute(context.Background(), operation, input)
		return operationResultMsg{viewID: viewID, frameID: frameID, operationID: operation.ID, input: input, result: result, err: err, background: background}
	}
}

func (model *Model) openStream(operation Operation, input RequestInput) tea.Cmd {
	model.cancelStream()
	ctx, cancel := context.WithCancel(context.Background())
	model.streamCancel = cancel
	model.loading = true
	frame := &model.frames[len(model.frames)-1]
	frame.ID = model.ensureFrameID(frame.ID)
	frame.InFlight = true
	viewID, frameID := frame.TargetViewID, frame.ID
	return func() tea.Msg {
		response, err := model.client.OpenStream(ctx, operation, input)
		if err != nil {
			return streamOpenedMsg{viewID: viewID, frameID: frameID, err: err}
		}
		events := make(chan streamEvent, 1)
		go pumpStream(ctx, response.Body, operation.Response.ContentType, events)
		return streamOpenedMsg{viewID: viewID, frameID: frameID, events: events}
	}
}

func waitStreamEvent(viewID string, frameID uint64, events <-chan streamEvent) tea.Cmd {
	return func() tea.Msg {
		event, open := <-events
		if !open {
			event.done = true
		}
		return streamEventMsg{viewID: viewID, frameID: frameID, events: events, event: event}
	}
}

func (model *Model) appendStreamEvent(value string) {
	model.DetailStreamComponent.Append(value)
}

func (model *Model) handleResult(message operationResultMsg) (tea.Model, tea.Cmd) {
	if model.currentView() == nil || len(model.frames) == 0 || message.viewID != model.currentView().ID || (message.frameID != 0 && message.frameID != model.frames[len(model.frames)-1].ID) {
		for index := range model.frames {
			if message.frameID != 0 && model.frames[index].ID == message.frameID {
				model.frames[index].InFlight = false
				model.frames[index].Refreshing = false
				break
			}
		}
		return model, nil
	}
	frame := &model.frames[len(model.frames)-1]
	frame.InFlight = false
	model.loading = false
	frame.Refreshing = false
	if message.err != nil {
		failedAction := model.actionInFlight
		if failedAction {
			model.restoreFailedActionWorkflow()
			model.presentRequestError("request:"+message.operationID, message.err, false)
			return model, nil
		}
		frame.Forbidden = strings.Contains(message.err.Error(), "HTTP 401") || strings.Contains(message.err.Error(), "HTTP 403")
		frame.LoadFailed = !message.background && len(model.rows) == 0 && frame.Selected == nil
		if message.background || !frame.LastSuccess.IsZero() {
			frame.Stale = true
			model.presentRequestError(model.refreshAlertKey(frame.ID), message.err, message.background)
		} else {
			model.presentRequestError("request:"+message.operationID, message.err, false)
		}
		return model, nil
	}
	operation := model.descriptor.Operation(message.operationID)
	if operation == nil {
		return model, nil
	}
	view := model.currentView()
	frame.RequestValues = captureRequestValues(*operation, message.input.Values)
	frame.RequestBody = decodeRuntimeBody(message.input.Body)
	frame.RequestURL = message.result.RequestURL
	frame.RequestMethod = message.result.RequestMethod
	frame.ResponseHeaders = message.result.Headers.Clone()
	frame.ResponseBody = message.result.Body
	frame.ResponseStatus = message.result.Status
	isRead := containsString(operation.Capabilities, "list") || containsString(operation.Capabilities, "get")
	if !isRead {
		model.actionInFlight = false
		model.form = nil
		model.confirmation = nil
		model.mode = modeBrowse
		model.shell.Modal.Close()
		model.shell.Alerts.Clear("request:" + message.operationID)
		model.alertSuccess("action", "Operation completed")
		return model, model.loadCurrent()
	}
	if containsString(operation.Capabilities, "list") && !operation.Response.Stream {
		items, err := responseItems(message.result.Body, operation.Response.ItemsPointer)
		if err != nil {
			model.markReadFailure(frame, message, err)
			return model, nil
		}
		model.setRows(*view, items)
		model.markReadSuccess(frame)
		model.shell.Alerts.Clear("request:" + message.operationID)
		return model, nil
	}
	if object, ok := message.result.Body.(map[string]any); ok {
		row := rowFor(*view, object)
		frame.Selected = &row
		model.ShowDetail(object, model.shell.Theme)
		model.mode = modeDetail
		model.markReadSuccess(frame)
		model.shell.Alerts.Clear("request:" + message.operationID)
		return model, nil
	}
	model.markReadFailure(frame, message, fmt.Errorf("operation %s returned no displayable object", message.operationID))
	return model, nil
}

func (model *Model) markReadSuccess(frame *Frame) {
	frame.LastSuccess = model.currentTime()
	frame.Stale, frame.Forbidden, frame.LoadFailed = false, false, false
	model.shell.Alerts.Clear(model.refreshAlertKey(frame.ID))
}

func (model *Model) markReadFailure(frame *Frame, message operationResultMsg, err error) {
	frame.Forbidden = false
	if message.background || !frame.LastSuccess.IsZero() || frame.Selected != nil {
		frame.Stale = true
		frame.LoadFailed = false
		model.alertError(model.refreshAlertKey(frame.ID), err.Error())
		return
	}
	frame.LoadFailed = true
	model.alertError("request:"+message.operationID, err.Error())
}

func (model *Model) rebuildTable(view View) {
	model.ensurePresentation()
	contentWidth, contentHeight := model.pageContentSize()
	offset := 0
	if len(model.frames) > 0 {
		offset = model.frames[len(model.frames)-1].ColumnOffset
		model.frames[len(model.frames)-1].ColumnOffset = model.ResourceTableComponent.Reset(view, model.shell.Theme, contentWidth, contentHeight, offset)
		return
	}
	model.ResourceTableComponent.Reset(view, model.shell.Theme, contentWidth, contentHeight, offset)
}

func (model *Model) configureTableColumns(view View) {
	contentWidth, contentHeight := model.pageContentSize()
	offset := 0
	if len(model.frames) > 0 {
		offset = model.frames[len(model.frames)-1].ColumnOffset
		model.frames[len(model.frames)-1].ColumnOffset = model.ResourceTableComponent.Configure(view, contentWidth, contentHeight, offset)
		return
	}
	model.ResourceTableComponent.Configure(view, contentWidth, contentHeight, offset)
}

func (model *Model) scrollColumns(direction int) {
	if len(model.frames) == 0 {
		return
	}
	frame := &model.frames[len(model.frames)-1]
	switch {
	case direction < 0 && model.leftOverflow > 0:
		frame.ColumnOffset--
	case direction > 0 && model.rightOverflow > 0:
		frame.ColumnOffset++
	default:
		return
	}
	view := model.currentView()
	if view == nil {
		return
	}
	model.configureTableColumns(*view)
	model.applyFilter()
}

func (model *Model) setRows(view View, items []map[string]any) {
	restoreIdentity := model.restoreIdentity
	if len(model.frames) > 0 {
		if model.frames[len(model.frames)-1].RefreshIdentity != "" {
			restoreIdentity = model.frames[len(model.frames)-1].RefreshIdentity
		}
		model.frames[len(model.frames)-1].RefreshIdentity = ""
		contentWidth, contentHeight := model.pageContentSize()
		offset := model.frames[len(model.frames)-1].ColumnOffset
		model.frames[len(model.frames)-1].ColumnOffset = model.ResourceTableComponent.SetRows(view, items, model.filter, restoreIdentity, contentWidth, contentHeight, offset)
		model.restoreIdentity = ""
		return
	}
	contentWidth, contentHeight := model.pageContentSize()
	model.ResourceTableComponent.SetRows(view, items, model.filter, restoreIdentity, contentWidth, contentHeight, 0)
	model.restoreIdentity = ""
}

func (model *Model) applyFilter() {
	model.ResourceTableComponent.ApplyFilter(model.filter)
}

func (model *Model) selectedRow() *Row {
	if model.currentView() != nil && model.currentView().Kind == "item" {
		return model.frames[len(model.frames)-1].Selected
	}
	return model.ResourceTableComponent.Selected()
}

func (model *Model) currentView() *View {
	if len(model.frames) == 0 {
		return nil
	}
	if model.frames[len(model.frames)-1].Catalog {
		view := resourceCatalogView()
		return &view
	}
	return model.descriptor.View(model.frames[len(model.frames)-1].TargetViewID)
}

func (model *Model) onCatalog() bool {
	return len(model.frames) > 0 && model.frames[len(model.frames)-1].Catalog
}

func (model *Model) addressableView(command string) *View {
	command = strings.ToLower(command)
	bindings := availableBindings(model.frames)
	for index := range model.descriptor.Views {
		view := &model.descriptor.Views[index]
		if view.ListOperationID == "" || !bindingsCover(view.ScopeParameters, bindings) {
			continue
		}
		if strings.EqualFold(view.Label, command) || strings.EqualFold(view.ID, command) {
			return view
		}
		for _, alias := range view.Aliases {
			if strings.EqualFold(alias, command) {
				return view
			}
		}
	}
	return nil
}

func (model *Model) resourceCommandCandidates() []string {
	bindings := availableBindings(model.frames)
	var candidates []string
	seen := make(map[string]bool)
	for index := range model.descriptor.Views {
		view := &model.descriptor.Views[index]
		if view.ListOperationID == "" || !bindingsCover(view.ScopeParameters, bindings) {
			continue
		}
		for _, candidate := range append([]string{view.Label, view.ID}, view.Aliases...) {
			candidate = strings.TrimSpace(SanitizeCell(candidate))
			key := strings.ToLower(candidate)
			if candidate != "" && !seen[key] {
				seen[key] = true
				candidates = append(candidates, candidate)
			}
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := strings.ToLower(candidates[i]), strings.ToLower(candidates[j])
		if left == right {
			return candidates[i] < candidates[j]
		}
		return left < right
	})
	return candidates
}

func (model *Model) breadcrumb() []BreadcrumbSegment {
	parts := make([]BreadcrumbSegment, 0, len(model.frames))
	for _, frame := range model.frames {
		parts = append(parts, BreadcrumbSegment{Label: frame.Label, Identity: frame.SelectedIdentity})
	}
	return parts
}

func (model *Model) resize() {
	if view := model.currentView(); view != nil {
		model.configureTableColumns(*view)
		model.applyFilter()
	}
	contentWidth, contentHeight := model.pageContentSize()
	model.DetailStreamComponent.Resize(contentWidth, contentHeight, model.shell.Theme)
}

func (model *Model) pageContentSize() (int, int) {
	model.ensurePresentation()
	view := model.currentView()
	if view == nil {
		layout := CalculateShellLayout(model.width, model.height, false, nil, nil)
		return layout.ContentWidth, layout.ContentHeight
	}
	presentation := model.shellView(*view, model.semanticPage(*view), model.commandPrompt())
	layout := model.shell.Layout(presentation, model.width, model.height)
	return layout.ContentWidth, layout.ContentHeight
}

func (model *Model) ensurePresentation() {
	if model.shell.Keys.bindings != nil {
		return
	}
	token := ""
	if model.client != nil {
		token = model.client.config.Token
	}
	model.shell = NewShell(token)
}

func (model *Model) serverOrigin() string {
	if model.client != nil {
		return model.client.config.BaseURL
	}
	if len(model.descriptor.Servers) > 0 {
		return model.descriptor.Servers[0].URL
	}
	return ""
}

func (model *Model) authenticated() bool {
	return model.client != nil && model.client.config.Token != ""
}

func presentationPulse() tea.Cmd {
	return tea.Tick(time.Second, func(now time.Time) tea.Msg { return presentationPulseMsg{now: now} })
}

func (model *Model) currentTime() time.Time {
	if model.now == nil {
		model.now = time.Now
	}
	return model.now()
}

func (model *Model) currentLastSuccess() time.Time {
	if len(model.frames) == 0 {
		return time.Time{}
	}
	return model.frames[len(model.frames)-1].LastSuccess
}

func (model *Model) currentRefreshing() bool {
	return len(model.frames) > 0 && model.frames[len(model.frames)-1].Refreshing
}

func (model *Model) updateStaleness(now time.Time) {
	if len(model.frames) == 0 || model.onCatalog() {
		return
	}
	frame := &model.frames[len(model.frames)-1]
	if frame.LastSuccess.IsZero() {
		return
	}
	threshold := 15 * time.Second
	if configured := 3 * model.refreshInterval; configured > threshold {
		threshold = configured
	}
	if now.Sub(frame.LastSuccess) > threshold {
		frame.Stale = true
	}
}

func (model *Model) newFrameID() uint64 {
	model.nextFrameID++
	return model.nextFrameID
}

func (model *Model) ensureFrameID(id uint64) uint64 {
	if id != 0 {
		return id
	}
	return model.newFrameID()
}

func (model *Model) refreshAlertKey(frameID uint64) string {
	return fmt.Sprintf("refresh:%d", frameID)
}

func (model *Model) clearFrameRequest(frameID uint64) {
	for index := range model.frames {
		if frameID != 0 && model.frames[index].ID == frameID {
			model.frames[index].InFlight = false
			model.frames[index].Refreshing = false
			return
		}
	}
}

func (model *Model) cancelStream() {
	model.DetailStreamComponent.Cancel()
}

func hasCollectionView(descriptor Descriptor) bool {
	for _, view := range descriptor.Views {
		if view.ListOperationID != "" {
			return true
		}
	}
	return false
}

func newChooser(columns []table.Column, rows []table.Row) table.Model {
	// Bubbles counts the header inside WithHeight, so reserve one additional
	// row to make every requested choice visible without an off-by-one clip.
	return table.New(table.WithColumns(columns), table.WithRows(rows), table.WithFocused(true), table.WithHeight(max(3, len(rows)+1)))
}

func evaluateBindings(edge Edge, frame Frame, row Row) (map[string]any, error) {
	result := cloneBindings(frame.Bindings)
	for _, binding := range edge.Bindings {
		var value any
		var err error
		switch binding.SourceKind {
		case "frame-path":
			value = frame.Bindings[binding.Source]
		case "row-property":
			value, err = ResolveJSONPointer(row.Raw, "/"+escapePointer(binding.Source))
		case "runtime-expression":
			value, err = evaluateExpression(binding.Source, frame, row)
		case "literal":
			value = binding.Source
		default:
			err = fmt.Errorf("unsupported binding source %s", binding.SourceKind)
		}
		if err != nil || value == nil || scalarString(value) == "" {
			if err == nil {
				err = fmt.Errorf("value is absent")
			}
			return nil, fmt.Errorf("relationship %s cannot bind %s: %w", edge.Name, binding.Target, err)
		}
		result[binding.Target] = value
	}
	return result, nil
}

func evaluateExpression(expression string, frame Frame, row Row) (any, error) {
	switch expression {
	case "$url":
		if frame.RequestURL != "" {
			return frame.RequestURL, nil
		}
		return nil, fmt.Errorf("request URL is absent")
	case "$method":
		if frame.RequestMethod != "" {
			return frame.RequestMethod, nil
		}
		return nil, fmt.Errorf("request method is absent")
	case "$statusCode":
		if frame.ResponseStatus != 0 {
			return frame.ResponseStatus, nil
		}
		return nil, fmt.Errorf("response status code is absent")
	}
	for _, prefix := range []string{"$request.path.", "$request.query.", "$request.header."} {
		if !strings.HasPrefix(expression, prefix) {
			continue
		}
		name := strings.TrimPrefix(expression, prefix)
		location := strings.TrimSuffix(strings.TrimPrefix(prefix, "$request."), ".")
		value, ok := requestExpressionValue(frame.RequestValues, location, name)
		if !ok && location == "path" {
			value, ok = frame.Bindings[name]
		}
		if !ok {
			return nil, fmt.Errorf("request %s parameter %s is absent", location, name)
		}
		return value, nil
	}
	if expression == "$request.body" || expression == "$request.body#" || strings.HasPrefix(expression, "$request.body#/") {
		pointer := strings.TrimPrefix(strings.TrimPrefix(expression, "$request.body"), "#")
		return resolveRuntimeBody(frame.RequestBody, pointer, "request")
	}
	if strings.HasPrefix(expression, "$response.header.") {
		name := strings.TrimPrefix(expression, "$response.header.")
		if value := frame.ResponseHeaders.Get(name); value != "" {
			return value, nil
		}
		return nil, fmt.Errorf("response header %s is absent", name)
	}
	if expression == "$response.body" || expression == "$response.body#" || strings.HasPrefix(expression, "$response.body#/") {
		body := frame.ResponseBody
		if body == nil {
			body = row.Raw
		}
		pointer := strings.TrimPrefix(strings.TrimPrefix(expression, "$response.body"), "#")
		return resolveRuntimeBody(body, pointer, "response")
	}
	return nil, fmt.Errorf("unsupported runtime expression %q", expression)
}

func captureRequestValues(operation Operation, values map[string]any) map[string]any {
	result := make(map[string]any)
	for _, parameter := range operation.Parameters {
		if value, ok := operationParameterValue(operation, parameter, values); ok {
			result[ParameterValueKey(parameter.In, parameter.Name)] = value
		}
	}
	return result
}

func requestExpressionValue(values map[string]any, location, name string) (any, bool) {
	if value, ok := values[ParameterValueKey(location, name)]; ok {
		return value, true
	}
	if location == "header" {
		prefix := location + "\x00"
		for key, value := range values {
			if strings.HasPrefix(key, prefix) && strings.EqualFold(strings.TrimPrefix(key, prefix), name) {
				return value, true
			}
		}
	}
	return nil, false
}

func decodeRuntimeBody(body []byte) any {
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return string(body)
	}
	return value
}

func resolveRuntimeBody(body any, pointer, label string) (any, error) {
	if body == nil {
		return nil, fmt.Errorf("%s body is absent", label)
	}
	return ResolveJSONPointer(body, pointer)
}

func responseItems(body any, pointer string) ([]map[string]any, error) {
	value := body
	var err error
	if pointer != "" {
		value, err = ResolveJSONPointer(body, pointer)
		if err != nil {
			return nil, err
		}
	}
	array, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("list response at %q is not an array", pointer)
	}
	items := make([]map[string]any, 0, len(array))
	for index, item := range array {
		object, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("list item %d is not an object", index)
		}
		items = append(items, object)
	}
	return items, nil
}

func pumpStream(ctx context.Context, reader io.ReadCloser, contentType string, events chan<- streamEvent) {
	defer close(events)
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	var data []string
	send := func(value string) bool {
		select {
		case events <- streamEvent{text: value}:
			return true
		case <-ctx.Done():
			return false
		}
	}
	flush := func() bool {
		if len(data) > 0 {
			if !send(strings.Join(data, "\n")) {
				return false
			}
			data = nil
		}
		return true
	}
	for scanner.Scan() {
		line := scanner.Text()
		if contentType == "text/event-stream" {
			if line == "" {
				if !flush() {
					return
				}
				continue
			}
			if strings.HasPrefix(line, "data:") {
				data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			}
			continue
		}
		if !send(line) {
			return
		}
	}
	if !flush() {
		return
	}
	if err := scanner.Err(); err != nil {
		select {
		case events <- streamEvent{err: fmt.Errorf("read stream: %w", err)}:
		case <-ctx.Done():
		}
		return
	}
	select {
	case events <- streamEvent{done: true}:
	case <-ctx.Done():
	}
}

func actionInputCount(operation Operation, values map[string]any) int {
	count := 0
	for _, parameter := range operation.Parameters {
		value, ok := operationParameterValue(operation, parameter, values)
		if !ok || strings.TrimSpace(scalarString(value)) == "" {
			count++
		}
	}
	if operation.RequestBody != nil {
		count++
	}
	return count
}

func actionLabel(operation Operation) string {
	if strings.TrimSpace(operation.Presentation.Label) != "" {
		return operation.Presentation.Label
	}
	if strings.TrimSpace(operation.Summary) != "" {
		return operation.Summary
	}
	return operation.ID
}

func (model *Model) operationsForCurrentView() []Operation {
	view := model.currentView()
	if view == nil {
		return nil
	}
	candidates := model.actionCandidates(*view, model.selectedRow())
	operations := make([]Operation, 0, len(candidates))
	for _, candidate := range candidates {
		operations = append(operations, candidate.Operation)
	}
	return operations
}

func (model *Model) actionCandidatesForCurrentView() []actionCandidate {
	view := model.currentView()
	if view == nil {
		return nil
	}
	return model.actionCandidates(*view, model.selectedRow())
}

func (model *Model) actionCandidates(view View, selected *Row) []actionCandidate {
	frame := Frame{Bindings: map[string]any{}}
	if len(model.frames) > 0 {
		frame = model.frames[len(model.frames)-1]
	}
	candidates := make([]actionCandidate, 0)
	seen := make(map[string]bool)
	appendView := func(candidateView View, values map[string]any) {
		for _, id := range candidateView.OperationIDs {
			operation := model.descriptor.Operation(id)
			if operation == nil || seen[id] || containsString(operation.Capabilities, "list") || containsString(operation.Capabilities, "get") {
				continue
			}
			seen[id] = true
			candidates = append(candidates, actionCandidate{Operation: *operation, Values: cloneBindings(values)})
		}
	}
	appendView(view, frame.Bindings)
	if view.Kind == "collection" && selected != nil {
		for _, edge := range model.descriptor.Outgoing(view.ID) {
			target := model.descriptor.View(edge.TargetViewID)
			if target == nil || target.Kind != "item" || target.SchemaRef != view.SchemaRef {
				continue
			}
			values, err := evaluateBindings(edge, frame, *selected)
			if err != nil {
				continue
			}
			appendView(*target, values)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Operation.ID < candidates[j].Operation.ID })
	return candidates
}

func (model *Model) alertInfo(key, summary string) {
	model.ensurePresentation()
	model.shell.Alerts.Push(key, AlertInfo, summary)
}

func (model *Model) alertSuccess(key, summary string) {
	model.ensurePresentation()
	model.shell.Alerts.Push(key, AlertSuccess, summary)
}

func (model *Model) alertWarning(key, summary string) {
	model.ensurePresentation()
	model.shell.Alerts.Push(key, AlertWarning, summary)
}

func (model *Model) alertError(key, summary string) {
	model.ensurePresentation()
	model.shell.Alerts.Push(key, AlertError, summary)
}

func (model *Model) presentRequestError(key string, requestErr error, background bool) {
	model.ensurePresentation()
	var apiError *APIError
	if !errors.As(requestErr, &apiError) {
		model.shell.Alerts.Push(key, AlertError, requestErr.Error())
		return
	}
	alert := model.shell.Alerts.Push(key, AlertError, apiError.Error(), apiError.Details())
	if background {
		return
	}
	model.previousMode = model.mode
	model.errorDialog = NewErrorDialog(apiError.DialogTitle(), apiError.DialogMessage(), apiError.DialogContext(), alert, model.shell.Keys)
	model.mode = modeErrorDialog
}

func (model *Model) handleErrorDialogMessage(message tea.Msg) (tea.Model, tea.Cmd) {
	if model.errorDialog == nil {
		return model, nil
	}
	closeDialog, command := model.errorDialog.Update(message)
	if closeDialog {
		model.mode = model.previousMode
		model.errorDialog = nil
		model.shell.Modal.Close()
	}
	return model, command
}

func (model *Model) restoreFailedActionWorkflow() {
	model.actionInFlight = false
	if model.form != nil {
		model.form.inFlight = false
		if len(model.form.fields) > 0 {
			model.confirmation = nil
			model.mode = modeActionInput
			return
		}
	}
	if model.confirmation != nil {
		model.confirmation.inFlight = false
		model.mode = modeConfirmation
		return
	}
	model.mode = modeBrowse
	model.shell.Modal.Close()
}

func availableBindings(frames []Frame) map[string]any {
	result := make(map[string]any)
	for _, frame := range frames {
		for name, value := range frame.Bindings {
			result[name] = value
		}
	}
	return result
}

func bindingsCover(names []string, values map[string]any) bool {
	for _, name := range names {
		if _, ok := values[name]; !ok {
			return false
		}
	}
	return true
}

func cloneBindings(values map[string]any) map[string]any {
	result := make(map[string]any, len(values))
	for name, value := range values {
		result[name] = value
	}
	return result
}

func escapePointer(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}
