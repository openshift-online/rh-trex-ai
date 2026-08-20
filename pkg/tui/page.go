package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
)

type PageState string

const (
	PageReady     PageState = "ready"
	PageLoading   PageState = "loading"
	PageEmpty     PageState = "empty"
	PageForbidden PageState = "forbidden"
	PageStale     PageState = "stale"
	PageFatal     PageState = "fatal"
)

type LocalAction struct {
	Label  string
	Hotkey string
}

// Page supplies only semantic content. The shell owns all outer chrome.
type Page interface {
	Title() string
	Scope() string
	Count() *int
	Filter() string
	SimpleTitle() bool
	State() PageState
	Content() string
	Actions() []LocalAction
}

type SemanticPage struct {
	PageTitle       string
	PageScope       string
	PageCount       *int
	PageFilter      string
	PageSimpleTitle bool
	PageState       PageState
	PageContent     string
	PageActions     []LocalAction
}

func (page SemanticPage) Title() string     { return page.PageTitle }
func (page SemanticPage) Scope() string     { return page.PageScope }
func (page SemanticPage) Count() *int       { return page.PageCount }
func (page SemanticPage) Filter() string    { return page.PageFilter }
func (page SemanticPage) SimpleTitle() bool { return page.PageSimpleTitle }
func (page SemanticPage) State() PageState  { return page.PageState }
func (page SemanticPage) Content() string   { return page.PageContent }
func (page SemanticPage) Actions() []LocalAction {
	return append([]LocalAction(nil), page.PageActions...)
}

type ResourceTablePage struct{ SemanticPage }
type DetailPage struct{ SemanticPage }
type StreamPage struct{ SemanticPage }

// ResourceTableComponent owns list presentation state for every descriptor.
// The application model supplies descriptors, frame offsets, and dimensions.
type ResourceTableComponent struct {
	rows            []Row
	visible         []Row
	displayColumns  []int
	columnWidths    []int
	leftOverflow    int
	rightOverflow   int
	table           table.Model
	filter          string
	restoreIdentity string
	sortProperty    string
	sortDescending  bool
	theme           Theme
}

// DetailStreamComponent owns the shared viewport and bounded stream state.
type DetailStreamComponent struct {
	detail       viewport.Model
	detailValue  any
	streamCancel context.CancelFunc
	streamEvents <-chan streamEvent
	streamLines  []string
	streamBytes  int
	autoscroll   bool
	connected    bool
}

type PageFrameTitle struct {
	Kind    string
	Context string
	Count   *int
	Filter  string
	Simple  bool
}

func pageFrameTitle(page Page) PageFrameTitle {
	context := SanitizeCell(page.Scope())
	if context == "" && !page.SimpleTitle() {
		context = "all"
	}
	return PageFrameTitle{
		Kind:    SanitizeCell(page.Title()),
		Context: context,
		Count:   page.Count(),
		Filter:  SanitizeCell(page.Filter()),
		Simple:  page.SimpleTitle(),
	}
}
