package globalsearch

import "github.com/yorukot/superfile/src/internal/common"

const firstResultRow = 3

// HandleClick handles a click using coordinates local to the modal.
func (m *Model) HandleClick(localY int) common.ModelAction {
	row := localY - firstResultRow
	if !m.open || row < 0 || row >= m.visibleResultCount() {
		return common.NoAction{}
	}

	index := m.renderIndex + row
	if index < 0 || index >= len(m.results) {
		return common.NoAction{}
	}

	selected := m.results[index]
	m.Close()
	return OpenResultAction{Result: selected}
}
