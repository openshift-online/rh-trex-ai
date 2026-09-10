package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
)

func (component *ResourceTableComponent) Reset(view View, theme Theme, width, height, offset int) int {
	component.sortProperty = view.DefaultSort
	component.sortDescending = false
	component.table = table.New(table.WithFocused(true), table.WithHeight(max(1, height)))
	component.theme = theme
	component.table.SetStyles(theme.TableStyles())
	component.rows = nil
	component.visible = nil
	return component.Configure(view, width, height, offset)
}

func (component *ResourceTableComponent) Configure(view View, width, height, offset int) int {
	component.table.SetWidth(max(1, width))
	if len(view.Columns) == 0 {
		component.displayColumns = nil
		component.columnWidths = nil
		component.leftOverflow = 0
		component.rightOverflow = 0
		component.table.SetHeight(max(1, height))
		component.replaceColumns([]table.Column{{Title: "VALUE", Width: max(1, width-2)}})
		return 0
	}
	displayView := view
	displayView.DefaultSort = component.sortProperty
	layout := calculateColumnLayout(displayView, component.rows, max(1, width-2), offset)
	component.displayColumns = layout.Visible
	component.columnWidths = layout.Widths
	component.leftOverflow = layout.LeftHidden
	component.rightOverflow = layout.RightHidden
	overflowHeight := 0
	if layout.LeftHidden > 0 || layout.RightHidden > 0 {
		overflowHeight = 1
	}
	component.table.SetHeight(max(1, height-overflowHeight))
	columns := make([]table.Column, 0, len(layout.Visible))
	for _, index := range layout.Visible {
		title := columnTitle(displayView, view.Columns[index])
		if view.Columns[index].Property == component.sortProperty && component.sortDescending {
			title = "↓ " + strings.TrimPrefix(title, "↑ ")
		}
		columns = append(columns, table.Column{Title: title, Width: layout.Widths[index]})
	}
	component.replaceColumns(columns)
	return layout.Offset
}

func (component *ResourceTableComponent) replaceColumns(columns []table.Column) {
	// Bubbles renders rows synchronously from SetColumns. Preserve the row count
	// with empty placeholders until ApplyFilter projects matching cells.
	if rowCount := len(component.table.Rows()); rowCount > 0 {
		component.table.SetRows(make([]table.Row, rowCount))
	}
	component.table.SetColumns(columns)
}

func (component *ResourceTableComponent) SetRows(view View, items []map[string]any, filter, restoreIdentity string, width, height, offset int) int {
	rows := make([]Row, 0, len(items))
	for _, item := range items {
		rows = append(rows, rowFor(view, item))
	}
	component.rows = rows
	component.sortRows()
	offset = component.Configure(view, width, height, offset)
	component.ApplyFilter(filter)
	if restoreIdentity != "" {
		for index := range component.visible {
			if component.visible[index].Identity == restoreIdentity {
				component.table.SetCursor(index)
				break
			}
		}
	}
	return offset
}

func (component *ResourceTableComponent) CycleSort(view View) {
	if len(view.Columns) == 0 {
		return
	}
	selected := component.Selected()
	identity := ""
	if selected != nil {
		identity = selected.Identity
	}
	index := -1
	for candidate := range view.Columns {
		if view.Columns[candidate].Property == component.sortProperty {
			index = candidate
			break
		}
	}
	index = (index + 1) % len(view.Columns)
	component.sortProperty = view.Columns[index].Property
	component.sortDescending = false
	component.sortRows()
	component.ApplyFilter(component.filter)
	component.restoreSelection(identity)
}

func (component *ResourceTableComponent) ReverseSort() {
	selected := component.Selected()
	identity := ""
	if selected != nil {
		identity = selected.Identity
	}
	component.sortDescending = !component.sortDescending
	component.sortRows()
	component.ApplyFilter(component.filter)
	component.restoreSelection(identity)
}

func (component *ResourceTableComponent) sortRows() {
	property := component.sortProperty
	if property == "" {
		return
	}
	sort.SliceStable(component.rows, func(i, j int) bool {
		left, _ := ResolveJSONPointer(component.rows[i].Raw, "/"+escapePointer(property))
		right, _ := ResolveJSONPointer(component.rows[j].Raw, "/"+escapePointer(property))
		less := scalarString(left) < scalarString(right)
		if component.sortDescending {
			return !less && scalarString(left) != scalarString(right)
		}
		return less
	})
}

func (component *ResourceTableComponent) restoreSelection(identity string) {
	if identity == "" {
		return
	}
	for index := range component.visible {
		if component.visible[index].Identity == identity {
			component.table.SetCursor(index)
			return
		}
	}
}

func (component *ResourceTableComponent) ApplyFilter(filter string) {
	component.filter = filter
	needle := strings.ToLower(SanitizeCell(filter))
	component.visible = nil
	var tableRows []table.Row
	for _, row := range component.rows {
		cells := component.visibleCells(row)
		filterCells := row.Cells
		if len(filterCells) == 0 {
			filterCells = []string{SanitizeCell(renderDetail(row.Raw))}
		}
		if needle != "" && !strings.Contains(strings.ToLower(strings.Join(filterCells, "\x00")), needle) {
			continue
		}
		component.visible = append(component.visible, row)
		tableRows = append(tableRows, table.Row(cells))
	}
	component.table.SetRows(tableRows)
	if len(tableRows) > 0 && component.table.Cursor() < 0 {
		component.table.SetCursor(0)
	}
}

func (component *ResourceTableComponent) visibleCells(row Row) []string {
	if len(component.displayColumns) == 0 {
		return []string{SanitizeCell(renderDetail(row.Raw))}
	}
	result := make([]string, 0, len(component.displayColumns))
	for _, index := range component.displayColumns {
		if index < len(row.Cells) {
			result = append(result, row.Cells[index])
		}
	}
	return result
}

func (component *ResourceTableComponent) Selected() *Row {
	index := component.table.Cursor()
	if index < 0 || index >= len(component.visible) {
		return nil
	}
	return &component.visible[index]
}

func (component *ResourceTableComponent) View() string {
	lines := strings.Split(component.table.View(), "\n")
	for index := 1; index < len(lines); index++ {
		lines[index] = component.theme.TableRow(lines[index])
	}
	result := strings.Join(lines, "\n")
	if overflow := renderColumnOverflow(component.leftOverflow, component.rightOverflow); overflow != "" {
		result += "\n" + overflow
	}
	return result
}

func rowFor(view View, item map[string]any) Row {
	row := Row{Raw: item}
	if view.IdentityProperty != "" {
		if value, err := ResolveJSONPointer(item, "/"+escapePointer(view.IdentityProperty)); err == nil {
			row.Identity = scalarString(value)
		}
	}
	for _, column := range view.Columns {
		value, _ := ResolveJSONPointer(item, "/"+escapePointer(column.Property))
		row.Cells = append(row.Cells, SanitizeCell(scalarString(value)))
	}
	return row
}
