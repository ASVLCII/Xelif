package internal

import "github.com/yorukot/superfile/src/internal/ui/filepanel"

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
