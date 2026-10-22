package internal

import tea "charm.land/bubbletea/v2"

const overlayWheelLines = 3

// Preserve overlay priority before the wheel can focus or scroll a main panel.
func (m *model) handleOverlayMouseWheel(msg tea.MouseWheelMsg) bool {
	switch {
	case m.spfError.IsOpen():
	case m.helpMenu.IsOpen():
		for range overlayWheelLines {
			switch msg.Button {
			case tea.MouseWheelUp:
				m.helpMenu.ListUp()
			case tea.MouseWheelDown:
				m.helpMenu.ListDown()
			}
		}
	case m.promptModal.IsOpen() || m.zoxideModal.IsOpen():
	case m.globalSearchModal.IsOpen():
		m.globalSearchModal.Scroll(mouseWheelDelta(msg.Button))
	case m.commandPalette.IsOpen():
		m.commandPalette.Scroll(mouseWheelDelta(msg.Button))
	case m.textViewer.IsOpen():
		if delta := mouseWheelDelta(msg.Button); delta != 0 {
			m.textViewer.Scroll(overlayWheelLines * delta)
		}
	case m.archiveViewer.IsOpen():
		if delta := mouseWheelDelta(msg.Button); delta != 0 {
			m.archiveViewer.Move(delta)
		}
	case m.remoteViewer.IsOpen():
		if delta := mouseWheelDelta(msg.Button); delta != 0 {
			m.remoteViewer.Move(delta)
		}
	default:
		return false
	}
	return true
}

func (m *model) handleViewerMouseClick(msg tea.MouseClickMsg) (tea.Cmd, bool) {
	switch {
	case m.textViewer.IsOpen():
		if msg.Button == tea.MouseLeft {
			m.handleTextViewerClick(msg.Mouse())
		}
	case m.archiveViewer.IsOpen():
		if msg.Button == tea.MouseLeft {
			return m.handleArchiveViewerClick(msg.Mouse()), true
		}
	case m.remoteViewer.IsOpen():
		if msg.Button == tea.MouseLeft {
			return m.handleRemoteViewerClick(msg.Mouse()), true
		}
	case m.globalSearchModal.IsOpen():
		if msg.Button == tea.MouseLeft {
			m.handleGlobalSearchClick(msg.Mouse())
			return nil, true
		}
	case m.commandPalette.IsOpen():
		if msg.Button == tea.MouseLeft {
			return m.handleCommandPaletteClick(msg.Mouse()), true
		}
	default:
		return nil, false
	}
	return nil, true
}
