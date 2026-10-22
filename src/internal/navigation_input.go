package internal

import tea "charm.land/bubbletea/v2"

func mouseWheelDelta(button tea.MouseButton) int {
	switch button {
	case tea.MouseWheelUp:
		return -1
	case tea.MouseWheelDown:
		return 1
	default:
		return 0
	}
}

func (m *model) navigateMouseHistory(forward bool) {
	panel := m.getFocusedFilePanel()
	var err error
	if forward {
		err = panel.Forward()
	} else {
		err = panel.Back()
	}
	if err != nil {
		m.showUserError("Could not navigate history", err)
	} else {
		m.recordDirectoryVisit(panel.Location)
	}
}

// A wheel over the tab row cycles that panel's tabs, including hidden tabs.
func (m *model) handleTabWheel(mouse tea.Mouse) bool {
	if mouse.Y != 1 || (mouse.Button != tea.MouseWheelUp && mouse.Button != tea.MouseWheelDown) {
		return false
	}
	start := m.sidebarModel.GetWidth()
	for index := range m.fileModel.FilePanels {
		panel := &m.fileModel.FilePanels[index]
		end := start + panel.GetWidth()
		if mouse.X > start && mouse.X < end-1 && mouse.X-start-1 < panel.TabBarWidth() {
			m.fileModel.FocusPanel(index)
			m.focusPanel = nonePanelFocus
			m.sidebarModel.SearchBarBlur()
			var err error
			if mouse.Button == tea.MouseWheelUp {
				err = panel.PreviousTab()
			} else {
				err = panel.NextTab()
			}
			if err != nil {
				m.showUserError("Could not switch tab", err)
			} else {
				m.recordDirectoryVisit(panel.Location)
			}
			return true
		}
		start = end
	}
	return false
}
