package filepanel

import (
	"sort"
	"strings"

	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

// ApplySortColumnAtContentX applies the sort represented by a column-header
// content coordinate.
func (m *Model) ApplySortColumnAtContentX(x int) bool {
	sortKind, sortable := m.SortKindAtContentX(x)
	if !sortable {
		return false
	}

	focusedPath := m.GetFocusedItem().Location
	reversed := false
	if m.SortKind == sortKind {
		reversed = !m.SortReversed
	}
	m.SortKind = sortKind
	m.SortReversed = reversed
	sort.Slice(m.element, getOrderingFunc(m.element, m.SortReversed, m.SortKind))
	m.scrollToCursor(m.FindElementIndexByLocation(focusedPath))
	return true
}

// SortKindAtContentX returns the sortable column represented by a file-panel
// content X coordinate.
func (m *Model) SortKindAtContentX(x int) (sortmodel.SortKind, bool) {
	columnStart := 0
	for _, column := range m.columns {
		columnEnd := columnStart + column.Size
		if x >= columnStart && x < columnEnd {
			return columnSortKind(column.Name)
		}
		columnStart = columnEnd
	}
	return 0, false
}

func columnSortKind(name string) (sortmodel.SortKind, bool) {
	switch strings.TrimSpace(name) {
	case "Name":
		return sortmodel.SortByName, true
	case "Size":
		return sortmodel.SortBySize, true
	case "Modified":
		return sortmodel.SortByDate, true
	default:
		return 0, false
	}
}
