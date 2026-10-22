package internal

import tea "charm.land/bubbletea/v2"

const keyboardContextMenuRow = 5

func (m *model) openKeyboardContextMenu() {
	if m.focusPanel == sidebarFocus {
		if path := m.sidebarModel.GetCurrentDirectoryLocation(); path != "" {
			m.contextMenu = contextMenuState{open: true, targetPath: path, sidebarTarget: true}
			m.positionContextMenu(tea.Mouse{X: 1, Y: keyboardContextMenuRow})
		}
		return
	}
	panel := m.getFocusedFilePanel()
	if m.focusPanel == nonePanelFocus && !panel.EmptyOrInvalid() {
		x := m.sidebarModel.GetWidth() + 1
		for i := range m.fileModel.FocusedPanelIndex {
			x += m.fileModel.FilePanels[i].GetWidth()
		}
		m.openContextMenu(
			tea.Mouse{X: x, Y: keyboardContextMenuRow},
			m.fileModel.FocusedPanelIndex,
			panel.GetFocusedItem().Location,
		)
	}
}

func (m *model) navigateListUp() {
	switch m.focusPanel {
	case sidebarFocus:
		m.sidebarModel.ListUp()
	case processBarFocus:
		m.processBarModel.ListUp()
	case metadataFocus:
		m.fileMetaData.ListUp()
	case nonePanelFocus:
		m.getFocusedFilePanel().ListUp()
	}
}

func (m *model) navigateListDown() {
	switch m.focusPanel {
	case sidebarFocus:
		m.sidebarModel.ListDown()
	case processBarFocus:
		m.processBarModel.ListDown()
	case metadataFocus:
		m.fileMetaData.ListDown()
	case nonePanelFocus:
		m.getFocusedFilePanel().ListDown()
	}
}

func (m *model) navigatePageUp() {
	switch m.focusPanel {
	case metadataFocus:
		m.fileMetaData.PgUp()
	case nonePanelFocus:
		m.getFocusedFilePanel().PgUp()
	case processBarFocus, sidebarFocus:
		// These panels intentionally do not page-scroll.
	}
}

func (m *model) navigatePageDown() {
	switch m.focusPanel {
	case metadataFocus:
		m.fileMetaData.PgDown()
	case nonePanelFocus:
		m.getFocusedFilePanel().PgDown()
	case processBarFocus, sidebarFocus:
		// These panels intentionally do not page-scroll.
	}
}

func (m *model) finishTabNavigation(err error, title string) {
	if err != nil {
		m.showUserError(title, err)
	} else {
		m.recordDirectoryVisit(m.getFocusedFilePanel().Location)
	}
}
