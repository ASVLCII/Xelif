package internal

import (
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
)

const contextMenuWidth = 22

const (
	contextOpen = iota
	contextCopy
	contextCut
	contextRename
	contextCopyPath
	contextTrash
	contextGitDiff
	contextGitStage
	contextGitUnstage
	contextGitDiscard
	contextSelectRange
)

var contextMenuItems = []string{ //nolint:gochecknoglobals // Static menu definition.
	"Open",
	"Copy",
	"Cut",
	"Rename",
	"Copy path",
	"Move to trash",
	"Git diff",
	"Git stage",
	"Git unstage",
	"Git discard",
	"Select range here",
}

type contextMenuState struct {
	open           bool
	x              int
	y              int
	panelIndex     int
	targetPath     string
	cursor         int
	rangeStartPath string
}

func (m *model) openContextMenu(mouse tea.Mouse, panelIndex int, targetPath string) {
	totalWidth, totalHeight := m.contextMenuDimensions()
	m.contextMenu = contextMenuState{
		open:       true,
		x:          min(max(mouse.X, 0), max(m.fullWidth-totalWidth, 0)),
		y:          min(max(mouse.Y, 0), max(m.fullHeight-totalHeight, 0)),
		panelIndex: panelIndex,
		targetPath: targetPath,
	}
}

func (m *model) closeContextMenu() {
	m.contextMenu = contextMenuState{}
}

func (m *model) contextMenuDimensions() (int, int) {
	rendered := m.contextMenuRender()
	return lipgloss.Width(rendered), lipgloss.Height(rendered)
}

func (m *model) contextMenuRender() string {
	lines := make([]string, len(contextMenuItems))
	for i, label := range contextMenuItems {
		prefix := "  "
		style := common.ModalStyle
		if i == m.contextMenu.cursor {
			prefix = "> "
			style = common.ModalCursorStyle
		}
		lines[i] = style.Render(prefix + label)
	}
	return common.SortOptionsModalBorderStyle(
		len(contextMenuItems), contextMenuWidth, common.Config.BorderBottom,
	).Render(strings.Join(lines, "\n"))
}

func (m *model) contextMenuClick(mouse tea.Mouse) (tea.Cmd, bool) {
	if !m.contextMenu.open {
		return nil, false
	}

	totalWidth, totalHeight := m.contextMenuDimensions()
	inside := mouse.X >= m.contextMenu.x && mouse.X < m.contextMenu.x+totalWidth &&
		mouse.Y >= m.contextMenu.y && mouse.Y < m.contextMenu.y+totalHeight
	if !inside {
		m.closeContextMenu()
		return nil, true
	}
	if mouse.X == m.contextMenu.x || mouse.X == m.contextMenu.x+totalWidth-1 {
		return nil, true
	}

	item := mouse.Y - m.contextMenu.y - 1 // account for top border
	if item < 0 || item >= len(contextMenuItems) {
		return nil, true
	}
	m.contextMenu.cursor = item
	return m.executeContextMenuAction(), true
}

func (m *model) contextMenuKey(key string) tea.Cmd {
	switch {
	case containsKey(common.Hotkeys.Quit, key), containsKey(common.Hotkeys.CancelTyping, key):
		m.closeContextMenu()
	case containsKey(common.Hotkeys.ListUp, key):
		m.contextMenu.cursor = (m.contextMenu.cursor - 1 + len(contextMenuItems)) % len(contextMenuItems)
	case containsKey(common.Hotkeys.ListDown, key):
		m.contextMenu.cursor = (m.contextMenu.cursor + 1) % len(contextMenuItems)
	case containsKey(common.Hotkeys.Confirm, key), containsKey(common.Hotkeys.ConfirmTyping, key):
		return m.executeContextMenuAction()
	}
	return nil
}

func containsKey(keys []string, key string) bool {
	for _, candidate := range keys {
		if candidate == key {
			return true
		}
	}
	return false
}

func (m *model) executeContextMenuAction() tea.Cmd {
	menu := m.contextMenu
	m.closeContextMenu()

	if !m.fileModel.FocusPanel(menu.panelIndex) {
		m.showUserError("Item is no longer available", errors.New("file panel was closed"))
		return nil
	}
	panel := m.getFocusedFilePanel()
	index := panel.FindElementIndexByLocation(menu.targetPath)
	if index < 0 || !panel.FocusIndex(index) {
		m.showUserError("Item is no longer available", errors.New("the folder changed; open the menu again"))
		return nil
	}
	m.focusPanel = nonePanelFocus

	switch menu.cursor {
	case contextSelectRange:
		start := panel.FindElementIndexByLocation(menu.rangeStartPath)
		if start >= 0 {
			panel.FocusIndex(start)
		}
		panel.ClearSelectionAnchor()
		if panel.SelectRangeTo(index) {
			panel.PanelMode = filepanel.SelectMode
		}
		panel.FocusIndex(index)
	case contextOpen:
		m.enterPanel()
	case contextCopy:
		if panel.PanelMode == filepanel.SelectMode {
			m.copyMultipleItem(false)
		} else {
			m.copySingleItem(false)
		}
	case contextCut:
		if panel.PanelMode == filepanel.SelectMode {
			m.copyMultipleItem(true)
		} else {
			m.copySingleItem(true)
		}
	case contextRename:
		m.panelItemRename()
	case contextCopyPath:
		m.copyPath()
	case contextTrash:
		return m.getDeleteTriggerCmd(false)
	case contextGitDiff:
		return m.gitDiffCmd(false, m.gitOperationPaths())
	case contextGitStage:
		return m.gitMutationCmd(gitMutationStage, m.gitOperationPaths())
	case contextGitUnstage:
		return m.gitMutationCmd(gitMutationUnstage, m.gitOperationPaths())
	case contextGitDiscard:
		return m.confirmGitDiscard(m.gitOperationPaths())
	}
	return nil
}
