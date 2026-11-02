package internal

import "charm.land/lipgloss/v2"

func (m *model) openSortMenu() {
	panel := m.getFocusedFilePanel()
	m.sortModal.Open(panel.SortKind, panel.SortReversed)
}

// Anchor the dropdown to its focused panel's toolbar, clamped to the screen.
func (m *model) sortMenuPosition() (int, int) {
	x := m.sidebarModel.GetWidth() + 1
	for _, index := range m.fileModel.VisiblePanelIndices() {
		if index == m.fileModel.FocusedPanelIndex {
			break
		}
		x += m.fileModel.FilePanels[index].GetWidth()
	}
	x += m.getFocusedFilePanel().SortActionContentX()
	width, height := lipgloss.Width(m.sortModal.Render()), lipgloss.Height(m.sortModal.Render())
	y := panelActionsRow + 1
	if len(m.workspaces) > 0 {
		y++
	}
	return min(max(x, 0), max(m.fullWidth-width, 0)), min(y, max(m.fullHeight-height, 0))
}
