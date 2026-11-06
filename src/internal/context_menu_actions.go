package internal

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/notify"
)

func contextSubmenu(id int) bool {
	return id == contextNew || id == contextArchives || id == contextView || id == contextMore
}

func (m *model) contextMenuActionIDs() []int {
	switch m.contextMenu.page {
	case contextNew:
		return []int{contextBack, contextNewFile, contextNewFolder}
	case contextArchives:
		ids := []int{contextBack, contextCompress}
		if m.contextTargetExtractable() {
			ids = append(ids, contextExtract)
		}
		return ids
	case contextView:
		return []int{contextBack, contextSort, contextHidden, contextPreview, contextFooter, contextRefresh}
	case contextMore:
		return []int{
			contextBack,
			contextSelectRange,
			contextGitDiff,
			contextGitStage,
			contextGitUnstage,
			contextGitDiscard,
		}
	}
	if m.contextMenu.background {
		return []int{
			contextNew,
			contextPaste,
			contextSort,
			contextView,
			contextRefresh,
			contextCopyPath,
			contextFavourite,
		}
	}
	ids := []int{contextOpen, contextCopy, contextCut, contextRename, contextCopyPath, contextTrash, contextNew}
	if m.contextTargetDirectory() {
		ids = append(ids, contextPaste, contextNewWorkspace, contextFavourite)
	} else {
		ids = append(ids, contextEditor)
	}
	return append(ids, contextArchives, contextView, contextProperties, contextMore)
}

func (m *model) contextTargetDirectory() bool {
	return m.contextMenu.targetDirectory
}

func (m *model) contextTargetExtractable() bool {
	return m.contextMenu.targetExtractable
}

func contextExtractablePath(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	for index := strings.IndexByte(name, '.'); index >= 0; index = strings.IndexByte(name, '.') {
		if common.IsExtensionExtractable(name[index:]) {
			return true
		}
		name = name[index+1:]
	}
	return false
}

func (m *model) contextActionLabel(id int) string {
	label := contextMenuItems[id]
	if id == contextCopyPath && m.contextMenu.background {
		return "Copy folder path"
	}
	if id == contextPaste {
		if len(m.clipboard.GetItems()) == 0 {
			return "Paste (clipboard empty)"
		}
		if !m.contextMenu.background {
			return "Paste into this folder"
		}
	}
	if id == contextNew && !m.contextMenu.background {
		return "New in current folder >"
	}
	return label
}

func (m *model) contextMenuShortcuts() []string {
	if m.contextMenu.workspaceTarget || m.contextMenu.sidebarTarget {
		return nil
	}
	ids := m.contextMenuActionIDs()
	shortcuts := make([]string, len(ids))
	for i, id := range ids {
		if id == contextCopyPath && m.contextMenu.background {
			continue
		}
		if keys := contextActionKeys(id); len(keys) > 0 {
			shortcuts[i] = keys[0]
		}
	}
	return shortcuts
}

func contextActionKeys(id int) []string {
	switch id {
	case contextOpen:
		return common.Hotkeys.Confirm
	case contextCopy:
		return common.Hotkeys.CopyItems
	case contextCut:
		return common.Hotkeys.CutItems
	case contextRename:
		return common.Hotkeys.FilePanelItemRename
	case contextCopyPath:
		return common.Hotkeys.CopyPath
	case contextTrash:
		return common.Hotkeys.DeleteItems
	case contextEditor:
		return common.Hotkeys.OpenFileWithEditor
	default:
		return nil
	}
}

func (m *model) clampContextMenu() {
	width, height := m.contextMenuDimensions()
	m.contextMenu.x = min(m.contextMenu.x, max(0, m.fullWidth-width))
	m.contextMenu.y = min(m.contextMenu.y, max(0, m.fullHeight-height))
}

func (m *model) contextMenuBack() {
	m.contextMenu.page = 0
	m.contextMenu.cursor = m.contextMenu.parentCursor
	m.clampContextMenu()
}

func (m *model) executeBackgroundContextMenu(menu contextMenuState) tea.Cmd {
	if cmd, handled := m.executeCommonContextAction(menu); handled {
		return cmd
	}
	switch menu.cursor {
	case contextCopyPath:
		m.copyPWD()
	case contextFavourite:
		m.toggleFavourite(menu.directory)
	}
	return nil
}

func (m *model) focusContextTarget(menu contextMenuState) (int, bool) {
	if !m.fileModel.FocusPanel(menu.panelIndex) {
		m.showUserError("Item is no longer available", errors.New("file panel was closed"))
		return 0, false
	}
	panel := m.getFocusedFilePanel()
	if menu.directory != "" && panel.Location != menu.directory {
		m.showUserError("Folder is no longer available", errors.New("the folder changed; open the menu again"))
		return 0, false
	}
	index := panel.FindElementIndexByLocation(menu.targetPath)
	if !menu.background && (index < 0 || !panel.FocusIndex(index)) {
		m.showUserError("Item is no longer available", errors.New("the folder changed; open the menu again"))
		return 0, false
	}
	if menu.cursor == contextTrash {
		entry := operationIdentity{entry: menu.identity.entry}
		if err := entry.check(menu.targetPath); err != nil {
			m.showUserError("Item changed or unavailable", err)
			return 0, false
		}
	}
	m.focusPanel = nonePanelFocus
	return index, true
}

func (m *model) executeCommonContextAction(menu contextMenuState) (tea.Cmd, bool) {
	switch menu.cursor {
	case contextNewFile, contextNewFolder:
		m.panelCreateNewFile()
		m.typingModal.directory = menu.cursor == contextNewFolder
		if m.typingModal.directory {
			m.typingModal.textInput.Placeholder = "Folder name"
		} else {
			m.typingModal.textInput.Placeholder = "File name"
		}
	case contextPaste:
		destination := menu.targetPath
		if menu.background {
			destination = menu.directory
		}
		return m.getPasteIntoSnapshotCmd(destination, menu.identity, menu.identityErr), true
	case contextSort:
		m.openSortMenu()
	case contextHidden:
		m.toggleDotFileController()
	case contextPreview:
		return m.fileModel.ToggleFilePreviewPanel(), true
	case contextFooter:
		return m.toggleFooterController(), true
	case contextRefresh:
		m.getFocusedFilePanel().UpdateElementsIfNeeded(true, m.fileModel.DisplayDotFiles)
	case contextEditor:
		return m.openFileWithEditor(), true
	case contextCompress:
		return m.getCompressSelectedFilesCmd(), true
	case contextExtract:
		return m.getExtractFileCmd(), true
	case contextProperties:
		m.showContextProperties()
	default:
		return nil, false
	}
	return nil, true
}

// Properties uses the displayed entry snapshot, not a recursive directory scan
// or an external metadata process. Opening it cannot block on an offline path.
func (m *model) showContextProperties() {
	item := m.getFocusedFilePanel().GetFocusedItem()
	if item.Info == nil {
		return
	}
	size := common.FormatFileSize(item.Info.Size())
	if item.Directory {
		size = "Not calculated (folder)"
	}
	path := strings.ReplaceAll(common.MakePrintable(item.Location), "\n", " ")
	path = common.TruncateTextBeginning(path, common.ModalWidth-common.InnerPadding, "...")
	content := fmt.Sprintf("%s\nSize: %s\nModified: %s\nMode: %s\n\nListed metadata; use Refresh to update.",
		path, size, item.Info.ModTime().Format("2006-01-02 15:04:05"), item.Info.Mode())
	m.notifyModel = notify.New(true, "Properties", content, notify.NoAction)
}
