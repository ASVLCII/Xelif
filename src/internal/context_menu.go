package internal

import (
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
)

const (
	contextMenuWidth       = 22
	contextMenuColumnGap   = 4
	contextMenuPrefixWidth = 2
	contextMenuMinLabel    = 8
)

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
	contextFavourite
	contextNewWorkspace
	contextPaste
	contextNew
	contextNewFile
	contextNewFolder
	contextArchives
	contextCompress
	contextExtract
	contextView
	contextHidden
	contextPreview
	contextFooter
	contextSort
	contextRefresh
	contextProperties
	contextMore
	contextBack
	contextEditor
)

var contextMenuItems = []string{ //nolint:gochecknoglobals // Static menu definition.
	"Open",
	"Copy",
	"Cut",
	"Rename",
	"Copy path",
	"Move to trash",
	gitDiffTitle,
	"Git stage",
	"Git unstage",
	"Git discard",
	"Select range here",
	"Toggle favourite",
	"Open in new workspace",
	"Paste",
	"New >",
	"New file",
	"New folder",
	"Archives >",
	"Compress...",
	"Extract...",
	"View >",
	"Show / hide hidden files",
	"Show / hide preview",
	"Show / hide information bar",
	"Sort by...",
	"Refresh",
	"Properties",
	"More actions >",
	"< Back",
	"Open in editor",
}

type contextMenuState struct {
	open              bool
	x                 int
	y                 int
	panelIndex        int
	targetPath        string
	cursor            int
	rangeStartPath    string
	sidebarTarget     bool
	workspaceTarget   bool
	workspaceTargetID string
	directory         string
	background        bool
	page              int
	parentCursor      int
	targetDirectory   bool
	targetExtractable bool
	identity          operationIdentity
	identityErr       error
}

func (m *model) openContextMenu(mouse tea.Mouse, panelIndex int, targetPath string) {
	if panelIndex < 0 || panelIndex >= len(m.fileModel.FilePanels) {
		return
	}
	m.contextMenu = contextMenuState{
		open:       true,
		panelIndex: panelIndex,
		targetPath: targetPath,
		directory:  m.fileModel.FilePanels[panelIndex].Location,
		background: targetPath == "",
	}
	panel := &m.fileModel.FilePanels[panelIndex]
	index := panel.FindElementIndexByLocation(targetPath)
	if index >= 0 {
		item := panel.GetElementAtIdx(index)
		if panel.PanelMode == filepanel.SelectMode && !panel.CheckSelected(targetPath) {
			panel.ResetSelected()
			panel.SetSelected(targetPath)
		}
		m.contextMenu.targetDirectory = item.Directory
		m.contextMenu.targetExtractable = !item.Directory && contextExtractablePath(targetPath)
	}
	identityPath := targetPath
	if identityPath == "" {
		identityPath = panel.Location
	}
	m.contextMenu.identity, m.contextMenu.identityErr = captureOperationIdentity(
		identityPath, m.contextMenu.background || m.contextMenu.targetDirectory,
	)
	m.positionContextMenu(mouse)
}

// openWorkspaceContextMenu targets a workspace by stable ID without activating it.
// The parent passes the raw strip click; positionContextMenu accounts for the strip row.
func (m *model) openWorkspaceContextMenu(mouse tea.Mouse, workspaceID string) {
	if m.workspaceIndexByID(workspaceID) < 0 {
		return
	}
	m.contextMenu = contextMenuState{open: true, workspaceTarget: true, workspaceTargetID: workspaceID}
	mouse.Y--
	m.positionContextMenu(mouse)
}

func (m *model) positionContextMenu(mouse tea.Mouse) {
	if len(m.workspaces) > 0 {
		mouse.Y++
	}
	totalWidth, totalHeight := m.contextMenuDimensions()
	m.contextMenu.x = min(max(mouse.X, 0), max(m.fullWidth-totalWidth, 0))
	m.contextMenu.y = min(max(mouse.Y, 0), max(m.fullHeight-totalHeight, 0))
}

func (m *model) contextMenuLabels() []string {
	if m.contextMenu.workspaceTarget {
		return []string{
			"Rename workspace",
			"Close workspace",
			"Move left",
			"Move right",
			"New workspace",
			"Reopen closed workspace",
		}
	}
	if m.contextMenu.sidebarTarget {
		return contextMenuItems[contextFavourite : contextFavourite+1]
	}
	ids := m.contextMenuActionIDs()
	labels := make([]string, len(ids))
	for i, id := range ids {
		labels[i] = m.contextActionLabel(id)
	}
	return labels
}

func (m *model) closeContextMenu() {
	m.contextMenu = contextMenuState{}
}

func (m *model) contextMenuDimensions() (int, int) {
	rendered := m.contextMenuRender()
	return lipgloss.Width(rendered), lipgloss.Height(rendered)
}

func (m *model) contextMenuRender() string {
	labels := m.contextMenuLabels()
	shortcuts := m.contextMenuShortcuts()
	width, shortcutWidth := contextMenuColumnWidths(labels, shortcuts, m.fullWidth-common.BorderPadding)
	lines := make([]string, len(labels))
	for i, label := range labels {
		prefix := "  "
		style := common.ModalStyle
		if i == m.contextMenu.cursor {
			prefix = "> "
			style = common.ModalCursorStyle
		}
		shortcut := ""
		if i < len(shortcuts) {
			shortcut = shortcuts[i]
		}
		line := contextMenuColumnLine(prefix, label, shortcut, width, shortcutWidth)
		lines[i] = style.Render(line)
	}
	return common.SortOptionsModalBorderStyle(
		len(labels), width+common.BorderPadding, common.Config.BorderBottom,
	).Render(strings.Join(lines, "\n"))
}

func contextMenuColumnWidths(labels, shortcuts []string, available int) (int, int) {
	labelWidth, shortcutWidth := 0, 0
	for _, label := range labels {
		labelWidth = max(labelWidth, ansi.StringWidth(label))
	}
	for _, shortcut := range shortcuts {
		shortcutWidth = max(shortcutWidth, ansi.StringWidth(shortcut))
	}
	width := contextMenuPrefixWidth + labelWidth + common.BorderPadding
	if shortcutWidth > 0 {
		width += contextMenuColumnGap + shortcutWidth
	}
	width = min(max(contextMenuWidth, width), max(1, available))
	// Keep actions legible when an unusually long custom binding cannot fit.
	// Omit the cue column rather than show a misleading partial shortcut.
	if width-contextMenuPrefixWidth-common.BorderPadding-contextMenuColumnGap-shortcutWidth < contextMenuMinLabel {
		shortcutWidth = 0
	}
	return width, shortcutWidth
}

func contextMenuColumnLine(prefix, label, shortcut string, width, shortcutWidth int) string {
	labelWidth := width - contextMenuPrefixWidth - common.BorderPadding
	if shortcutWidth > 0 {
		labelWidth -= contextMenuColumnGap + shortcutWidth
	}
	label = ansi.Truncate(label, max(0, labelWidth), "...")
	line := prefix + label
	if shortcutWidth > 0 && shortcut != "" {
		gap := width - common.BorderPadding - ansi.StringWidth(line) - ansi.StringWidth(shortcut)
		line += strings.Repeat(" ", max(contextMenuColumnGap, gap)) + shortcut
	}
	line = ansi.Truncate(line, width, "")
	return line + strings.Repeat(" ", max(0, width-ansi.StringWidth(line)))
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
	if item < 0 || item >= len(m.contextMenuLabels()) {
		return nil, true
	}
	m.contextMenu.cursor = item
	return m.executeContextMenuAction(), true
}

func (m *model) contextMenuKey(key string) tea.Cmd {
	switch {
	case key == "esc":
		if m.contextMenu.page != 0 {
			m.contextMenuBack()
		} else {
			m.closeContextMenu()
		}
	case containsKey(common.Hotkeys.Quit, key), containsKey(common.Hotkeys.CancelTyping, key):
		m.closeContextMenu()
	case containsKey(common.Hotkeys.ListUp, key):
		count := len(m.contextMenuLabels())
		m.contextMenu.cursor = (m.contextMenu.cursor - 1 + count) % count
	case containsKey(common.Hotkeys.ListDown, key):
		m.contextMenu.cursor = (m.contextMenu.cursor + 1) % len(m.contextMenuLabels())
	case containsKey(common.Hotkeys.Confirm, key), containsKey(common.Hotkeys.ConfirmTyping, key):
		return m.executeContextMenuAction()
	case key == appearanceLeftKey:
		if m.contextMenu.page != 0 {
			m.contextMenuBack()
		}
	case key == "right":
		ids := m.contextMenuActionIDs()
		if !m.contextMenu.workspaceTarget && !m.contextMenu.sidebarTarget &&
			m.contextMenu.cursor >= 0 && m.contextMenu.cursor < len(ids) && contextSubmenu(ids[m.contextMenu.cursor]) {
			return m.executeContextMenuAction()
		}
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
	if !menu.workspaceTarget && !menu.sidebarTarget {
		ids := m.contextMenuActionIDs()
		if menu.cursor < 0 || menu.cursor >= len(ids) {
			return nil
		}
		id := ids[menu.cursor]
		if id == contextBack {
			m.contextMenuBack()
			return nil
		}
		if contextSubmenu(id) {
			m.contextMenu.page = id
			m.contextMenu.parentCursor = menu.cursor
			m.contextMenu.cursor = 1
			m.clampContextMenu()
			return nil
		}
		if id == contextPaste && len(m.clipboard.GetItems()) == 0 {
			return nil
		}
		menu.cursor = id // Dispatch stable action identity, not its visible row.
	}
	m.closeContextMenu()
	if menu.workspaceTarget {
		return m.executeWorkspaceContextMenu(menu)
	}
	if menu.sidebarTarget {
		m.toggleFavourite(menu.targetPath)
		return nil
	}
	return m.executeFileContextMenu(menu)
}

func (m *model) executeWorkspaceContextMenu(menu contextMenuState) tea.Cmd {
	actions := [...]string{
		workspaceActionRename,
		workspaceActionClose,
		workspaceActionLeft,
		workspaceActionRight,
		workspaceActionNew,
		workspaceActionReopen,
	}
	if menu.cursor >= 0 && menu.cursor < len(actions) {
		return m.applyWorkspaceContextCommand(menu.workspaceTargetID, actions[menu.cursor])
	}
	return nil
}

func (m *model) executeFileContextMenu(menu contextMenuState) tea.Cmd {
	index, valid := m.focusContextTarget(menu)
	if !valid {
		return nil
	}
	if menu.background {
		return m.executeBackgroundContextMenu(menu)
	}
	panel := m.getFocusedFilePanel()
	if cmd, handled := m.executeCommonContextAction(menu); handled {
		return cmd
	}

	switch menu.cursor {
	case contextNewWorkspace:
		if panel.GetFocusedItem().Directory {
			return m.newWorkspace(menu.targetPath)
		}
		m.showUserError("Choose a folder", errors.New("only folders can open a new workspace"))
	case contextFavourite:
		m.toggleFavourite(menu.targetPath)
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
		identity := operationIdentity{entry: menu.identity.entry}
		return m.prepareDeleteConfirmation(false, map[string]operationIdentity{menu.targetPath: identity})
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
