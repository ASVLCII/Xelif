package internal

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/common"
)

func (m *model) navigateFilePanel(msg string, delta int) tea.Cmd {
	arrow := "shift+right"
	if delta < 0 {
		arrow = "shift+left"
	}
	if len(m.workspaces) > 0 && msg == arrow {
		return m.moveBrowserFocus(delta)
	}
	if m.focusPanel == nonePanelFocus {
		if delta > 0 {
			m.fileModel.NextFilePanel()
		} else {
			m.fileModel.PreviousFilePanel()
		}
	}
	return nil
}

// Move through the visible browser zones in screen order. Unlike Tab's legacy
// file-panel cycling, directional arrows stop at the outside edges.
func (m *model) moveBrowserFocus(delta int) tea.Cmd {
	index := m.fileModel.FocusedPanelIndex
	switch m.focusPanel {
	case sidebarFocus:
		if delta < 0 {
			return nil
		}
		index = 0
	case nonePanelFocus:
		index += delta
	case processBarFocus, metadataFocus:
		return nil
	}
	if index < 0 {
		if common.Config.SidebarWidth == 0 {
			return nil
		}
		if m.fullWidth < sidebarDrawerBreakpoint {
			return m.openSidebarDrawer()
		}
		if !m.sidebarModel.Disabled() {
			m.focusOnSideBar()
		}
		return nil
	}
	if m.fileModel.FocusPanel(index) {
		m.focusPanel = nonePanelFocus
		m.sidebarModel.SearchBarBlur()
	}
	return nil
}

// Pinned directory
func (m *model) pinnedDirectory() {
	panel := m.getFocusedFilePanel()
	path := panel.Location
	if m.focusPanel == sidebarFocus {
		if selected := m.sidebarModel.GetCurrentDirectoryLocation(); selected != "" {
			path = selected
		}
	}
	m.toggleFavourite(path)
}

func (m *model) toggleFavourite(path string) {
	err := m.sidebarModel.TogglePinnedDirectory(path)
	if err != nil {
		slog.Error("Error while toggling pinned directory", "error", err)
		m.showUserError("Could not update Favourites", err)
	}
}

// Focus on sidebar
func (m *model) focusOnSideBar() {
	if common.Config.SidebarWidth == 0 {
		return
	}
	if m.focusPanel == sidebarFocus {
		m.focusPanel = nonePanelFocus
		m.getFocusedFilePanel().IsFocused = true
	} else {
		m.focusPanel = sidebarFocus
		m.getFocusedFilePanel().IsFocused = false
	}
}

// Focus on processbar
func (m *model) focusOnProcessBar() {
	if !m.toggleFooter {
		return
	}

	if m.focusPanel == processBarFocus {
		m.focusPanel = nonePanelFocus
		m.getFocusedFilePanel().IsFocused = true
	} else {
		m.focusPanel = processBarFocus
		m.getFocusedFilePanel().IsFocused = false
	}
}

// focus on metadata
func (m *model) focusOnMetadata() {
	if !m.toggleFooter {
		return
	}

	if m.focusPanel == metadataFocus {
		m.focusPanel = nonePanelFocus
		m.getFocusedFilePanel().IsFocused = true
	} else {
		m.focusPanel = metadataFocus
		m.getFocusedFilePanel().IsFocused = false
	}
}
