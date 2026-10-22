package commandpalette

import "github.com/yorukot/superfile/src/internal/common"

const firstItemRow = 3

// HandleClick handles a click using coordinates local to the palette.
func (m *Model) HandleClick(localY int) common.ModelAction {
	row := localY - firstItemRow
	if !m.open || row < 0 || row >= m.visibleItemCount() {
		return common.NoAction{}
	}
	index := m.renderIndex + row
	if index < 0 || index >= len(m.filtered) || m.filtered[index].Disabled {
		return common.NoAction{}
	}
	selected := m.filtered[index]
	m.Close()
	return ExecuteAction{ID: selected.ID}
}
