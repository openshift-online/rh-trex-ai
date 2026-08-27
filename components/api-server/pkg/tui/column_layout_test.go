package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDisplayCellWidthHandlesUnicodeGraphemes(t *testing.T) {
	tests := map[string]struct {
		value string
		want  int
	}{
		"ascii":     {value: "hello", want: 5},
		"combining": {value: "e\u0301", want: 1},
		"cjk":       {value: "界", want: 2},
		"emoji":     {value: "🙂", want: 2},
	}
	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			if got := displayCellWidth(testCase.value); got != testCase.want {
				t.Fatalf("display width of %q = %d, want %d", testCase.value, got, testCase.want)
			}
		})
	}
}

func TestContentWidthsAreSemanticAndStable(t *testing.T) {
	view := View{DefaultSort: "count", Columns: []Column{
		{Property: "count", Label: "COUNT", Priority: 100, Type: "integer"},
		{Property: "id", Label: "ID", Priority: 90, Type: "string", Format: "uuid"},
		{Property: "description", Label: "DESCRIPTION", Priority: 10, Type: "string"},
		{Property: "unicode", Label: "UNICODE", Priority: 20, Type: "string"},
	}}
	rows := []Row{
		{Cells: []string{"7", "agent-123", "short", "e\u0301"}},
		{Cells: []string{"42", "agent-123456789", "a considerably longer explanation", "界🙂"}},
	}
	layout := calculateColumnLayout(view, rows, 96, 0)
	if len(layout.Visible) != len(view.Columns) || layout.LeftHidden != 0 || layout.RightHidden != 0 {
		t.Fatalf("fitting layout = %#v", layout)
	}
	if layout.Widths[0] >= layout.Widths[2] {
		t.Fatalf("numeric width %d should be smaller than text width %d", layout.Widths[0], layout.Widths[2])
	}
	if layout.Widths[1] >= layout.Widths[2] {
		t.Fatalf("identifier width %d should be smaller than text width %d", layout.Widths[1], layout.Widths[2])
	}

	repeated := calculateColumnLayout(view, rows, 96, 0)
	if !reflect.DeepEqual(layout.Widths, repeated.Widths) {
		t.Fatalf("same unfiltered rows changed widths: %v != %v", layout.Widths, repeated.Widths)
	}
}

func TestFillWidthSingleColumnReachesTableEdge(t *testing.T) {
	view := View{
		FillWidth: true,
		Columns:   []Column{{Property: "resource", Label: "RESOURCE", Type: "string"}},
	}
	layout := calculateColumnLayout(view, []Row{{Cells: []string{"Dinosaurs"}}}, 96, 0)
	if got := columnsDisplayWidth(layout.Widths, 0, len(layout.Widths)); got != 96 {
		t.Fatalf("fill-width table uses %d cells, want 96", got)
	}
	if !reflect.DeepEqual(layout.Visible, []int{0}) || layout.LeftHidden != 0 || layout.RightHidden != 0 {
		t.Fatalf("fill-width layout = %#v", layout)
	}
}

func TestPriorityControlsCompressionWithoutRemovingColumns(t *testing.T) {
	view := View{Columns: []Column{
		{Property: "low", Label: "LOW", Priority: 1, Type: "string"},
		{Property: "high", Label: "HIGH", Priority: 100, Type: "string"},
		{Property: "medium", Label: "MEDIUM", Priority: 50, Type: "string"},
	}}
	rows := []Row{{Cells: []string{strings.Repeat("l", 20), strings.Repeat("h", 20), strings.Repeat("m", 20)}}}
	layout := calculateColumnLayout(view, rows, 36, 0)
	if !reflect.DeepEqual(layout.Visible, []int{0, 1, 2}) {
		t.Fatalf("compressed columns = %v, want every declaration", layout.Visible)
	}
	if !(layout.Widths[0] < layout.Widths[2] && layout.Widths[2] < layout.Widths[1]) {
		t.Fatalf("priority widths = %v, want low < medium < high", layout.Widths)
	}
}

func TestHorizontalLayoutReportsReachableOverflow(t *testing.T) {
	view := View{Columns: []Column{
		{Property: "a", Label: "A", Type: "integer"},
		{Property: "b", Label: "B", Type: "integer"},
		{Property: "c", Label: "C", Type: "integer"},
		{Property: "d", Label: "D", Type: "integer"},
		{Property: "e", Label: "E", Type: "integer"},
		{Property: "f", Label: "F", Type: "integer"},
	}}
	left := calculateColumnLayout(view, nil, 20, 0)
	if !reflect.DeepEqual(left.Visible, []int{0, 1, 2}) || left.LeftHidden != 0 || left.RightHidden != 3 {
		t.Fatalf("left layout = %#v", left)
	}
	middle := calculateColumnLayout(view, nil, 20, 1)
	if !reflect.DeepEqual(middle.Visible, []int{1, 2, 3}) || middle.LeftHidden != 1 || middle.RightHidden != 2 {
		t.Fatalf("middle layout = %#v", middle)
	}
	right := calculateColumnLayout(view, nil, 20, 3)
	if !reflect.DeepEqual(right.Visible, []int{3, 4, 5}) || right.LeftHidden != 3 || right.RightHidden != 0 {
		t.Fatalf("right layout = %#v", right)
	}
	resized := calculateColumnLayout(view, nil, 40, right.Offset)
	if !reflect.DeepEqual(resized.Visible, []int{0, 1, 2, 3, 4, 5}) || resized.Offset != 0 {
		t.Fatalf("resized layout = %#v", resized)
	}
}

func TestOverflowAffordanceAndKeyRegistry(t *testing.T) {
	if got := renderColumnOverflow(0, 3); !strings.Contains(got, "3 ▶") || strings.Contains(got, "◀") || !strings.Contains(got, columnScrollHint()) {
		t.Fatalf("right affordance = %q", got)
	}
	if got := renderColumnOverflow(2, 3); !strings.Contains(got, "◀ 2") || !strings.Contains(got, "3 ▶") {
		t.Fatalf("middle affordance = %q", got)
	}
	if got := renderColumnOverflow(2, 0); !strings.Contains(got, "◀ 2") || strings.Contains(got, "▶") {
		t.Fatalf("left affordance = %q", got)
	}
	if got := renderColumnOverflow(0, 0); got != "" {
		t.Fatalf("fitting affordance = %q", got)
	}
	if direction := columnScrollDirection(tea.KeyMsg{Type: tea.KeyLeft}); direction != -1 {
		t.Fatalf("left direction = %d", direction)
	}
	if direction := columnScrollDirection(tea.KeyMsg{Type: tea.KeyRight}); direction != 1 {
		t.Fatalf("right direction = %d", direction)
	}
}

func TestTableTruncatesAtDisplayWidthAndKeepsFullDetailValue(t *testing.T) {
	value := "界界界界界界界界"
	view := View{
		ID: "records", Kind: "collection", Label: "Records", IdentityProperty: "id",
		Columns: []Column{{Property: "description", Label: "DESCRIPTION", Type: "string"}},
	}
	model := &Model{
		descriptor: Descriptor{Title: "Test", Views: []View{view}},
		width:      14,
		height:     20,
		frames:     []Frame{{TargetViewID: view.ID, Label: view.Label, Bindings: map[string]any{}}},
	}
	model.rebuildTable(view)
	item := map[string]any{"id": "record-1", "description": value}
	model.setRows(view, []map[string]any{item})

	output := model.tableView()
	if !strings.Contains(output, "…") || strings.Contains(output, value) {
		t.Fatalf("bounded table cell was not ellipsized: %q", output)
	}
	if detail := renderDetail(item); !strings.Contains(detail, value) {
		t.Fatalf("detail lost complete cell value: %q", detail)
	}
}
