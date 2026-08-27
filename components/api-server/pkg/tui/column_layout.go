package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	tableColumnGutterWidth    = 2
	tableColumnMinimumWidth   = 4
	tableColumnHeaderLimit    = 12
	tableColumnCompactMaximum = 20
	tableColumnIDMaximum      = 36
	tableColumnTextMaximum    = 48
)

type columnLayout struct {
	Widths      []int
	Visible     []int
	Offset      int
	LeftHidden  int
	RightHidden int
}

type columnPolicy struct {
	minimum         int
	maximum         int
	expansionWeight int
}

func calculateColumnLayout(view View, rows []Row, available, requestedOffset int) columnLayout {
	count := len(view.Columns)
	if count == 0 {
		return columnLayout{}
	}
	available = max(1, available)
	widths := make([]int, count)
	minimums := make([]int, count)
	maximums := make([]int, count)
	weights := make([]int, count)

	for index, column := range view.Columns {
		titleWidth := displayCellWidth(columnTitle(view, column))
		policy := sizingPolicy(column, titleWidth)
		policy.maximum = min(policy.maximum, max(1, available-tableColumnGutterWidth))
		policy.minimum = min(policy.minimum, policy.maximum)
		natural := titleWidth
		for _, row := range rows {
			if index < len(row.Cells) {
				natural = max(natural, displayCellWidth(row.Cells[index]))
			}
		}
		minimums[index] = policy.minimum
		maximums[index] = policy.maximum
		weights[index] = policy.expansionWeight
		widths[index] = clampInt(natural, policy.minimum, policy.maximum)
	}
	if view.FillWidth && count == 1 {
		fillWidth := max(1, available-tableColumnGutterWidth)
		minimums[0] = min(minimums[0], fillWidth)
		maximums[0] = fillWidth
		widths[0] = fillWidth
	}

	used := columnsDisplayWidth(widths, 0, len(widths))
	if used < available {
		expandColumns(widths, maximums, weights, available-used)
	} else if used > available {
		compressColumns(view.Columns, widths, minimums, used-available)
	}

	return visibleColumnLayout(widths, available, requestedOffset)
}

func sizingPolicy(column Column, titleWidth int) columnPolicy {
	minimum := max(tableColumnMinimumWidth, min(titleWidth, tableColumnHeaderLimit))
	property := strings.ToLower(strings.ReplaceAll(column.Property, "-", "_"))
	compactName := property == "id" || strings.HasSuffix(property, "_id") || property == "status" ||
		property == "state" || property == "phase" || property == "kind" || property == "type"

	switch column.Type {
	case "boolean", "integer", "number":
		return columnPolicy{minimum: minimum, maximum: max(minimum, tableColumnCompactMaximum)}
	case "string":
		switch {
		case column.Format == "uuid" || property == "id" || strings.HasSuffix(property, "_id"):
			return columnPolicy{minimum: minimum, maximum: max(minimum, tableColumnIDMaximum)}
		case column.Format == "date" || column.Format == "date-time" || compactName:
			return columnPolicy{minimum: minimum, maximum: max(minimum, tableColumnCompactMaximum)}
		default:
			return columnPolicy{minimum: minimum, maximum: max(minimum, tableColumnTextMaximum), expansionWeight: 2}
		}
	default:
		if compactName {
			return columnPolicy{minimum: minimum, maximum: max(minimum, tableColumnIDMaximum)}
		}
		return columnPolicy{minimum: minimum, maximum: max(minimum, tableColumnTextMaximum), expansionWeight: 1}
	}
}

func expandColumns(widths, maximums, weights []int, remaining int) {
	for remaining > 0 {
		progress := false
		for index, weight := range weights {
			for range weight {
				if remaining == 0 || widths[index] >= maximums[index] {
					break
				}
				widths[index]++
				remaining--
				progress = true
			}
		}
		if !progress {
			return
		}
	}
}

func compressColumns(columns []Column, widths, minimums []int, deficit int) {
	order := make([]int, len(columns))
	for index := range order {
		order[index] = index
	}
	sort.SliceStable(order, func(i, j int) bool {
		left, right := order[i], order[j]
		if columns[left].Priority != columns[right].Priority {
			return columns[left].Priority < columns[right].Priority
		}
		return left > right
	})
	for _, index := range order {
		available := widths[index] - minimums[index]
		shrink := min(deficit, available)
		widths[index] -= shrink
		deficit -= shrink
		if deficit == 0 {
			return
		}
	}
}

func visibleColumnLayout(widths []int, available, requestedOffset int) columnLayout {
	if len(widths) == 0 {
		return columnLayout{}
	}
	if columnsDisplayWidth(widths, 0, len(widths)) <= available {
		visible := make([]int, len(widths))
		for index := range visible {
			visible[index] = index
		}
		return columnLayout{Widths: widths, Visible: visible}
	}

	offset := clampInt(requestedOffset, 0, len(widths)-1)
	for offset > 0 && columnsDisplayWidth(widths, offset-1, len(widths)) <= available {
		offset--
	}
	visible := visibleColumnsFrom(widths, offset, available)
	last := offset + len(visible)
	return columnLayout{
		Widths: widths, Visible: visible, Offset: offset,
		LeftHidden: offset, RightHidden: len(widths) - last,
	}
}

func visibleColumnsFrom(widths []int, offset, available int) []int {
	remaining := available
	visible := make([]int, 0, len(widths)-offset)
	for index := offset; index < len(widths); index++ {
		columnWidth := widths[index] + tableColumnGutterWidth
		if columnWidth > remaining {
			break
		}
		visible = append(visible, index)
		remaining -= columnWidth
	}
	if len(visible) == 0 {
		visible = append(visible, offset)
	}
	return visible
}

func columnsDisplayWidth(widths []int, start, end int) int {
	total := 0
	for _, width := range widths[start:end] {
		total += width + tableColumnGutterWidth
	}
	return total
}

func columnTitle(view View, column Column) string {
	title := SanitizeCell(column.Label)
	if column.Property == view.DefaultSort {
		title = "↑ " + title
	}
	return title
}

func displayCellWidth(value string) int {
	return lipgloss.Width(SanitizeCell(value))
}

func columnScrollDirection(message tea.KeyMsg) int {
	registry := DefaultKeyRegistry()
	switch {
	case registry.Matches(message, KeyColumnsLeft):
		return -1
	case registry.Matches(message, KeyColumnsRight):
		return 1
	default:
		return 0
	}
}

func columnScrollHint() string {
	return DefaultKeyRegistry().ColumnHint()
}

func renderColumnOverflow(left, right int) string {
	if left == 0 && right == 0 {
		return ""
	}
	parts := make([]string, 0, 3)
	if left > 0 {
		parts = append(parts, fmt.Sprintf("◀ %d", left))
	}
	parts = append(parts, columnScrollHint())
	if right > 0 {
		parts = append(parts, fmt.Sprintf("%d ▶", right))
	}
	return strings.Join(parts, " ")
}

func clampInt(value, low, high int) int {
	return min(max(value, low), high)
}
