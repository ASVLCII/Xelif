package internal

import (
	"errors"
	"log/slog"
	"slices"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/filemodel"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
	"github.com/yorukot/superfile/src/internal/ui/spferror"

	"github.com/yorukot/superfile/src/internal/ui/notify"

	tea "charm.land/bubbletea/v2"

	variable "github.com/yorukot/superfile/src/config"
)

// mainKey handles most of key commands in the regular state of the application. For
// keys that performs actions in multiple panels, like going up or down,
// check the state of model m and handle properly.
// Keep binding checks flat and ordered: custom hotkeys can overlap, and the
// first matching action must win. Helpers handle focus-dependent operations.
//
//nolint:gocyclo,cyclop,funlen // Ordered binding dispatch preserves custom-key precedence.
func (m *model) mainKey(msg string) tea.Cmd {
	switch {
	case msg == "shift+f10", msg == "menu":
		m.openKeyboardContextMenu()
	// If move up Key is pressed, check the current state and executes
	case slices.Contains(common.Hotkeys.ListUp, msg):
		m.navigateListUp()

		// If move down Key is pressed, check the current state and executes
	case slices.Contains(common.Hotkeys.ListDown, msg):
		m.navigateListDown()

	case slices.Contains(common.Hotkeys.PageUp, msg):
		m.navigatePageUp()

	case slices.Contains(common.Hotkeys.PageDown, msg):
		m.navigatePageDown()

	case slices.Contains(common.Hotkeys.ChangePanelMode, msg):
		m.getFocusedFilePanel().ChangeFilePanelMode()

	case slices.Contains(common.Hotkeys.NextFilePanel, msg):
		if m.focusPanel == nonePanelFocus {
			m.fileModel.NextFilePanel()
		}

	case slices.Contains(common.Hotkeys.PreviousFilePanel, msg):
		if m.focusPanel == nonePanelFocus {
			m.fileModel.PreviousFilePanel()
		}

	case slices.Contains(common.Hotkeys.CloseFilePanel, msg):
		cmd, err := m.fileModel.CloseFilePanel()
		if err != nil && !errors.Is(err, filemodel.ErrMinimumPanelCount) {
			slog.Error("unexpected error while closing new panel", "error", err)
		}
		return cmd
	case slices.Contains(common.Hotkeys.CreateNewFilePanel, msg):
		cmd, err := m.createNewFilePanel(variable.HomeDir)
		if err != nil && !errors.Is(err, filemodel.ErrMaximumPanelCount) {
			slog.Error("unexpected error while creating new panel", "error", err)
		}
		return cmd
	case slices.Contains(common.Hotkeys.SplitFilePanel, msg):
		cmd, err := m.splitPanel()
		if err != nil && !errors.Is(err, filemodel.ErrMaximumPanelCount) {
			slog.Error("unexpected error while splitting panel", "error", err)
		}
		return cmd
	case slices.Contains(common.Hotkeys.ToggleFilePreviewPanel, msg):
		return m.fileModel.ToggleFilePreviewPanel()
	case slices.Contains(common.Hotkeys.NewFileTab, msg):
		if err := m.getFocusedFilePanel().NewTab(m.getFocusedFilePanel().Location); err != nil {
			m.showUserError("Could not open tab", err)
		}
	case slices.Contains(common.Hotkeys.CloseFileTab, msg):
		m.finishTabNavigation(m.getFocusedFilePanel().CloseTab(), "Could not close tab")
	case slices.Contains(common.Hotkeys.NextFileTab, msg):
		m.finishTabNavigation(m.getFocusedFilePanel().NextTab(), "Could not switch tab")
	case slices.Contains(common.Hotkeys.PreviousFileTab, msg):
		m.finishTabNavigation(m.getFocusedFilePanel().PreviousTab(), "Could not switch tab")
	case slices.Contains(common.Hotkeys.HistoryBack, msg):
		m.finishTabNavigation(m.getFocusedFilePanel().Back(), "Could not go back")
	case slices.Contains(common.Hotkeys.HistoryForward, msg):
		m.finishTabNavigation(m.getFocusedFilePanel().Forward(), "Could not go forward")

	case slices.Contains(common.Hotkeys.FocusOnSidebar, msg):
		m.focusOnSideBar()

	case slices.Contains(common.Hotkeys.FocusOnProcessBar, msg):
		m.focusOnProcessBar()

	case slices.Contains(common.Hotkeys.FocusOnMetaData, msg):
		m.focusOnMetadata()

	case slices.Contains(common.Hotkeys.PasteItems, msg):
		return m.getPasteItemCmd()
	case slices.Contains(common.Hotkeys.UndoLastFileOperation, msg):
		return m.getUndoLastFileOperationCmd()

	case slices.Contains(common.Hotkeys.FilePanelItemCreate, msg):
		m.panelCreateNewFile()
	case slices.Contains(common.Hotkeys.PinnedDirectory, msg):
		m.pinnedDirectory()

	case slices.Contains(common.Hotkeys.ToggleDotFile, msg):
		m.toggleDotFileController()

	case slices.Contains(common.Hotkeys.ToggleFooter, msg):
		return m.toggleFooterController()

	case slices.Contains(common.Hotkeys.ExtractFile, msg):
		return m.getExtractFileCmd()

	case slices.Contains(common.Hotkeys.CompressFile, msg):
		return m.getCompressSelectedFilesCmd()

	case slices.Contains(common.Hotkeys.OpenCommandLine, msg):
		m.promptModal.Open(true)
	case slices.Contains(common.Hotkeys.OpenSPFPrompt, msg):
		m.promptModal.Open(false)
	case slices.Contains(common.Hotkeys.OpenZoxide, msg):
		return m.zoxideModal.Open()
	case slices.Contains(common.Hotkeys.OpenGlobalSearch, msg):
		return m.globalSearchModal.Open(m.getFocusedFilePanel().Location)
	case slices.Contains(common.Hotkeys.OpenCommandPalette, msg):
		return m.openCommandPalette()

	case slices.Contains(common.Hotkeys.OpenHelpMenu, msg):
		m.helpMenu.Open()

	case slices.Contains(common.Hotkeys.OpenSortOptionsMenu, msg):
		m.sortModal.Open(m.getFocusedFilePanel().SortKind)

	case slices.Contains(common.Hotkeys.ToggleReverseSort, msg):
		m.getFocusedFilePanel().ToggleReverseSort()

	case slices.Contains(common.Hotkeys.OpenFileWithEditor, msg):
		return m.openFileWithEditor()

	case slices.Contains(common.Hotkeys.OpenCurrentDirectoryWithEditor, msg):
		return m.openDirectoryWithEditor()

	default:
		return m.normalAndBrowserModeKey(msg)
	}

	return nil
}

func (m *model) normalAndBrowserModeKey(msg string) tea.Cmd {
	// if not focus on the filepanel return
	if !m.getFocusedFilePanel().IsFocused {
		m.unfocusedFilePanelKey(msg)
		return nil
	}
	// Check if in the select mode and focusOn filepanel
	if m.getFocusedFilePanel().PanelMode == filepanel.SelectMode {
		return m.filePanelSelectModeKey(msg)
	}

	return m.filePanelNormalModeKey(msg)
}

func (m *model) unfocusedFilePanelKey(msg string) {
	if m.focusPanel != sidebarFocus {
		return
	}

	if slices.Contains(common.Hotkeys.Confirm, msg) {
		m.sidebarSelectDirectory()
	}
	if slices.Contains(common.Hotkeys.FilePanelItemRename, msg) {
		m.sidebarModel.PinnedItemRename()
	}
	if slices.Contains(common.Hotkeys.SearchBar, msg) {
		m.sidebarSearchBarFocus()
	}
}

func (m *model) filePanelSelectModeKey(msg string) tea.Cmd {
	panel := m.getFocusedFilePanel()

	switch {
	case slices.Contains(common.Hotkeys.Confirm, msg):
		panel.SingleItemSelect()
	case slices.Contains(common.Hotkeys.FilePanelSelectModeItemsSelectUp, msg):
		panel.ItemSelectUp()
	case slices.Contains(common.Hotkeys.FilePanelSelectModeItemsSelectDown, msg):
		panel.ItemSelectDown()
	case slices.Contains(common.Hotkeys.DeleteItems, msg):
		return m.getDeleteTriggerCmd(false)
	case slices.Contains(common.Hotkeys.PermanentlyDeleteItems, msg):
		return m.getDeleteTriggerCmd(true)
	case slices.Contains(common.Hotkeys.CopyItems, msg):
		m.copyMultipleItem(false)
	case slices.Contains(common.Hotkeys.CutItems, msg):
		m.copyMultipleItem(true)
	case slices.Contains(common.Hotkeys.CopyPath, msg):
		m.copyPath()
	case slices.Contains(common.Hotkeys.FilePanelSelectAllItem, msg):
		panel.SelectAllItem()
	case slices.Contains(common.Hotkeys.FilePanelSelectRange, msg):
		panel.SelectRangeTo(panel.GetCursor())
	case slices.Contains(common.Hotkeys.FilePanelInvertSelection, msg):
		panel.InvertVisibleSelection()
	case slices.Contains(common.Hotkeys.FilePanelSelectFiles, msg):
		panel.SelectVisibleFiles()
	case slices.Contains(common.Hotkeys.FilePanelSelectDirectories, msg):
		panel.SelectVisibleDirectories()
	}
	return nil
}

func (m *model) filePanelNormalModeKey(msg string) tea.Cmd {
	switch {
	case slices.Contains(common.Hotkeys.Confirm, msg):
		m.enterPanel()
	case slices.Contains(common.Hotkeys.ParentDirectory, msg):
		m.parentDirectory()
	case slices.Contains(common.Hotkeys.DeleteItems, msg):
		return m.getDeleteTriggerCmd(false)
	case slices.Contains(common.Hotkeys.PermanentlyDeleteItems, msg):
		return m.getDeleteTriggerCmd(true)
	case slices.Contains(common.Hotkeys.CopyItems, msg):
		m.copySingleItem(false)
	case slices.Contains(common.Hotkeys.CutItems, msg):
		m.copySingleItem(true)
	case slices.Contains(common.Hotkeys.FilePanelItemRename, msg):
		m.panelItemRename()
	case slices.Contains(common.Hotkeys.SearchBar, msg):
		m.searchBarFocus()
	case slices.Contains(common.Hotkeys.CopyPath, msg):
		m.copyPath()
	case slices.Contains(common.Hotkeys.CopyPWD, msg):
		m.copyPWD()
	}
	return nil
}

// Check the hotkey to cancel operation or create file
func (m *model) typingModalOpenKey(msg string) tea.Cmd {
	switch {
	case slices.Contains(common.Hotkeys.CancelTyping, msg):
		m.cancelTypingModal()
	case slices.Contains(common.Hotkeys.ConfirmTyping, msg):
		return m.getCreateCmd()
	}
	return nil
}

func (m *model) notifyModelOpenKey(msg string) tea.Cmd {
	isCancel := slices.Contains(common.Hotkeys.CancelTyping, msg) || slices.Contains(common.Hotkeys.Quit, msg)
	isConfirm := slices.Contains(common.Hotkeys.ConfirmTyping, msg)

	if !isCancel && !isConfirm {
		slog.Warn("Invalid keypress in notifyModel", "msg", msg)
		return nil
	}
	m.notifyModel.Close()
	action := m.notifyModel.GetConfirmAction()
	if isCancel {
		return m.handleNotifyModelCancel(action)
	}
	return m.handleNotifyModelConfirm(action)
}

func (m *model) handleNotifyModelCancel(action notify.ConfirmActionType) tea.Cmd {
	switch action {
	case notify.RenameAction:
		m.cancelRename()
	case notify.QuitAction:
		m.modelQuitState = notQuitting
	case notify.GitDiscardAction:
		m.pendingGitDiscard = nil
	case notify.DropPasteAction:
		m.pendingDroppedPaths = nil
	case notify.DeleteAction, notify.NoAction, notify.PermanentDeleteAction:
		// Do nothing
	default:
		slog.Error("Unknown type of action", "action", action)
	}
	return nil
}

func (m *model) handleNotifyModelConfirm(action notify.ConfirmActionType) tea.Cmd {
	switch action {
	case notify.DeleteAction:
		return m.getDeleteCmd(false)
	case notify.PermanentDeleteAction:
		return m.getDeleteCmd(true)
	case notify.RenameAction:
		m.confirmRename()
	case notify.QuitAction:
		m.modelQuitState = quitConfirmationReceived
	case notify.NoAction:
		// Ignore
	case notify.GitDiscardAction:
		paths := append([]string(nil), m.pendingGitDiscard...)
		m.pendingGitDiscard = nil
		return m.gitMutationCmd(gitMutationDiscard, paths)
	case notify.DropPasteAction:
		paths := append([]string(nil), m.pendingDroppedPaths...)
		m.pendingDroppedPaths = nil
		m.clipboard.Reset(false)
		m.clipboard.SetItems(paths)
		return m.getPasteItemCmd()
	default:
		slog.Error("Unknown type of action", "action", action)
	}
	return nil
}

func (m *model) spfErrorModelOpenKey(msg string) tea.Cmd {
	isAbort := slices.Contains(spferror.KeyAbort(), msg)
	isSkip := slices.Contains(spferror.KeySkip(), msg)

	if !isAbort && !isSkip {
		slog.Warn("Invalid keypress in spfErrorModel", "msg", msg)
		return nil
	}
	defer func() {
		slog.Debug("Unlock mutex for modal error window")
		m.mutexErrorModal.Unlock()
	}()
	state := m.spfError.Close()
	if state == nil {
		return nil
	}
	reqID := m.nextIoReqCnt()
	if isSkip {
		return func() tea.Msg { return state.Skip(m.runFileProcessor, reqID) }
	}
	return func() tea.Msg { return state.Abort(m.runFileProcessor, reqID) }
}

// Handles key inputs inside sort options menu
func (m *model) sortOptionsKey(msg string) {
	switch {
	case slices.Contains(common.Hotkeys.OpenSortOptionsMenu, msg):
		m.sortModal.Close()
	case slices.Contains(common.Hotkeys.Quit, msg):
		m.sortModal.Close()
	case slices.Contains(common.Hotkeys.Confirm, msg):
		m.confirmSortOptions()
	case slices.Contains(common.Hotkeys.ListUp, msg):
		m.sortModal.ListUp()
	case slices.Contains(common.Hotkeys.ListDown, msg):
		m.sortModal.ListDown()
	}
}

func (m *model) renamingKey(msg string) {
	switch {
	case slices.Contains(common.Hotkeys.CancelTyping, msg):
		m.cancelRename()
	case slices.Contains(common.Hotkeys.ConfirmTyping, msg):
		m.confirmRename()
	}
}

func (m *model) sidebarRenamingKey(msg string) {
	switch {
	case slices.Contains(common.Hotkeys.CancelTyping, msg):
		m.sidebarModel.CancelSidebarRename()
	case slices.Contains(common.Hotkeys.ConfirmTyping, msg):
		m.sidebarModel.ConfirmSidebarRename()
	}
}

// Check the key input and cancel or confirms the search
func (m *model) focusOnSearchbarKey(msg string) {
	switch {
	case slices.Contains(common.Hotkeys.CancelTyping, msg):
		m.cancelSearch()
	case slices.Contains(common.Hotkeys.ConfirmTyping, msg):
		m.confirmSearch()
	}
}
