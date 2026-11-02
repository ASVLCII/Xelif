package internal

import (
	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/ui/filepanel"
)

const (
	panelActionsRow = 2
	panelFilterRow  = 3
)

func (m *model) handlePanelHeaderClick(panel *filepanel.Model, contentX int) {
	if create, closeTab := panel.TabControlAtContentX(contentX); create || closeTab {
		var err error
		if create {
			err = panel.NewTab(panel.Location)
		} else {
			err = panel.CloseTab()
		}
		if err != nil {
			m.showUserError("Could not change tabs", err)
		} else {
			m.recordDirectoryVisit(panel.Location)
		}
		return
	}
	if tabIndex, ok := panel.TabAtContentX(contentX); ok {
		if err := panel.ActivateTab(tabIndex); err != nil {
			m.showUserError("Could not switch tab", err)
		} else {
			m.recordDirectoryVisit(panel.Location)
		}
		return
	}
	if path, ok := panel.BreadcrumbPathAtContentX(contentX); ok {
		if err := m.updateCurrentFilePanelDir(path); err != nil {
			m.showUserError("Could not open folder", err)
		}
	}
}

func (m *model) handlePanelToolbarClick(panel *filepanel.Model, x, y int) tea.Cmd {
	search, commands := panel.SearchActionAtContentX(x)
	switch {
	case search:
		return m.globalSearchModal.Open(panel.Location)
	case commands:
		return m.openCommandPalette()
	case panel.SortActionAtContentX(x):
		m.openSortMenu()
	case y == panel.FilterRow() && x < panel.FilterTextWidth()+1:
		panel.SearchBar.Focus()
		m.firstTextInput = true
	}
	return nil
}
