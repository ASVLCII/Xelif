package internal

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"
)

func wheelMainAction(button tea.MouseButton, m *model) {
	slog.Debug("wheelMainAction called", "button", button, "focusPanel", m.focusPanel)
	var action func()
	switch button {
	case tea.MouseWheelUp:
		switch m.focusPanel {
		case sidebarFocus:
			action = func() { m.sidebarModel.Scroll(-1) }
		case processBarFocus:
			action = func() { m.processBarModel.ListUp() }
		case metadataFocus:
			action = func() { m.fileMetaData.ListUp() }
		case nonePanelFocus:
			action = func() {
				panel := m.getFocusedFilePanel()
				panel.FocusIndex(panel.GetCursor() - 1)
			}
		}

	case tea.MouseWheelDown:
		switch m.focusPanel {
		case sidebarFocus:
			action = func() { m.sidebarModel.Scroll(1) }
		case processBarFocus:
			action = func() { m.processBarModel.ListDown() }
		case metadataFocus:
			action = func() { m.fileMetaData.ListDown() }
		case nonePanelFocus:
			action = func() {
				panel := m.getFocusedFilePanel()
				panel.FocusIndex(panel.GetCursor() + 1)
			}
		}
	default:
		slog.Error("Unexpected type of mouse action in wheelMainAction", "button", button)
		return
	}

	if action != nil {
		action()
	}
}
