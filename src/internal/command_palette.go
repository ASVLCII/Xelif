package internal

import (
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/archivebrowser"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/commandpalette"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
	"github.com/yorukot/superfile/src/internal/ui/prompt"
	"github.com/yorukot/superfile/src/pkg/utils"
)

const (
	commandIDOperations          = "operations"
	commandIDOpen                = "open"
	commandIDParent              = "parent"
	commandIDGlobalSearch        = "global_search"
	commandIDZoxide              = "zoxide"
	commandIDNewPanel            = "new_panel"
	commandIDSplitPanel          = "split_panel"
	commandIDClosePanel          = "close_panel"
	commandIDNewTab              = "new_tab"
	commandIDCloseTab            = "close_tab"
	commandIDNextTab             = "next_tab"
	commandIDPreviousTab         = "previous_tab"
	commandIDHistoryBack         = "history_back"
	commandIDHistoryForward      = "history_forward"
	commandIDTogglePreview       = "toggle_preview"
	commandIDToggleFooter        = "toggle_footer"
	commandIDToggleDotFiles      = "toggle_dot_files"
	commandIDToggleSelection     = "toggle_selection"
	commandIDSelectAll           = "select_all"
	commandIDSelectRange         = "select_range"
	commandIDInvertSelection     = "invert_selection"
	commandIDSelectFiles         = "select_files"
	commandIDSelectDirectories   = "select_directories"
	commandIDSort                = "sort"
	commandIDReverseSort         = "reverse_sort"
	commandIDCreate              = "create"
	commandIDRename              = "rename"
	commandIDBatchRenamePrompt   = "batch_rename_prompt"
	commandIDBatchRenameApply    = "batch_rename_apply"
	commandIDBatchRenameCancel   = "batch_rename_cancel"
	commandIDCopy                = "copy"
	commandIDCut                 = "cut"
	commandIDPaste               = "paste"
	commandIDUndo                = "undo"
	commandIDTrash               = "trash"
	commandIDDelete              = "delete_permanently"
	commandIDCopyPath            = "copy_path"
	commandIDCopyDirectory       = "copy_directory"
	commandIDFavorite            = "favorite"
	commandIDCompress            = "compress"
	commandIDExtract             = "extract"
	commandIDBrowseArchive       = "browse_archive"
	commandIDBrowseRemote        = "browse_remote"
	commandIDGitDiff             = "git_diff"
	commandIDGitStagedDiff       = "git_staged_diff"
	commandIDGitStage            = "git_stage"
	commandIDGitUnstage          = "git_unstage"
	commandIDGitDiscard          = "git_discard"
	commandIDFindDuplicates      = "find_duplicates"
	commandIDCancelDuplicates    = "cancel_duplicate_scan"
	commandIDCompareDirectories  = "compare_directories"
	commandIDCompareContent      = "compare_directories_content"
	commandIDPreviewCopyMissing  = "preview_copy_missing"
	commandIDApplyCopyMissing    = "apply_copy_missing"
	commandIDCancelCompare       = "cancel_directory_compare"
	commandIDMetadataPrompt      = "metadata_prompt"
	commandIDMetadataApply       = "metadata_apply"
	commandIDMetadataRestore     = "metadata_restore"
	commandIDOpenEditor          = "open_editor"
	commandIDOpenDirectoryEditor = "open_directory_editor"
	commandIDCommandLine         = "command_line"
	commandIDSPFPrompt           = "spf_prompt"
	commandIDWorkspacePrompt     = "workspace_prompt"
	commandIDSavedSearchPrompt   = "saved_search_prompt"
	commandIDCatalogPrompt       = "catalog_prompt"
	commandIDCollectionPrompt    = "collection_prompt"
	commandIDHelp                = "help"
	commandIDFocusSidebar        = "focus_sidebar"
	commandIDFocusProcesses      = "focus_processes"
	commandIDFocusMetadata       = "focus_metadata"
	commandIDQuit                = "quit"
	commandIDWorkspaceLoadPrefix = "workspace_load:"
	commandIDSavedSearchPrefix   = "saved_search_run:"
	commandIDTagFindPrefix       = "tag_find:"
	commandIDCollectionPrefix    = "collection_open:"
)

func (m *model) openCommandPalette() tea.Cmd {
	m.commandPalette = commandpalette.New(m.paletteItems())
	m.setCommandPaletteSize()
	return m.commandPalette.Open()
}

func (m *model) paletteItems() []commandpalette.Item {
	panel := m.getFocusedFilePanel()
	hasFocusedItem := !panel.Empty()
	hasOperationItems := hasFocusedItem
	if panel.PanelMode == filepanel.SelectMode {
		hasOperationItems = panel.SelectedCount() > 0
	}

	items := []commandpalette.Item{
		operationsPaletteItem(),
		paletteItem(commandIDOpen, "Open focused item", "Files", common.Hotkeys.Confirm, "enter", "launch"),
		paletteItem(commandIDParent, "Open parent folder", "Navigation", common.Hotkeys.ParentDirectory, "up"),
		paletteItem(commandIDGlobalSearch, "Search everywhere", "Search", common.Hotkeys.OpenGlobalSearch,
			"everything"),
		paletteItem(commandIDZoxide, "Open zoxide navigation", "Search", common.Hotkeys.OpenZoxide, "jump"),
		paletteItem(commandIDNewPanel, "New file panel", "Panels", common.Hotkeys.CreateNewFilePanel),
		paletteItem(commandIDSplitPanel, "Split current panel", "Panels", common.Hotkeys.SplitFilePanel),
		paletteItem(commandIDClosePanel, "Close current panel", "Panels", common.Hotkeys.CloseFilePanel),
		paletteItem(commandIDNewTab, "New tab", "Tabs", common.Hotkeys.NewFileTab),
		paletteItem(commandIDCloseTab, "Close tab", "Tabs", common.Hotkeys.CloseFileTab),
		paletteItem(commandIDNextTab, "Next tab", "Tabs", common.Hotkeys.NextFileTab),
		paletteItem(commandIDPreviousTab, "Previous tab", "Tabs", common.Hotkeys.PreviousFileTab),
		paletteItem(commandIDHistoryBack, "Go back", "Navigation", common.Hotkeys.HistoryBack),
		paletteItem(commandIDHistoryForward, "Go forward", "Navigation", common.Hotkeys.HistoryForward),
		paletteItem(commandIDTogglePreview, "Toggle preview", "View", common.Hotkeys.ToggleFilePreviewPanel),
		paletteItem(commandIDToggleFooter, "Toggle footer", "View", common.Hotkeys.ToggleFooter),
		paletteItem(commandIDToggleDotFiles, "Toggle hidden files", "View", common.Hotkeys.ToggleDotFile,
			"dotfiles"),
		paletteItem(commandIDToggleSelection, "Toggle selection mode", "Selection", common.Hotkeys.ChangePanelMode),
		paletteItem(commandIDSelectAll, "Select all visible items", "Selection",
			common.Hotkeys.FilePanelSelectAllItem),
		paletteItem(commandIDSelectRange, "Select range to focused item", "Selection",
			common.Hotkeys.FilePanelSelectRange),
		paletteItem(commandIDInvertSelection, "Invert visible selection", "Selection",
			common.Hotkeys.FilePanelInvertSelection),
		paletteItem(commandIDSelectFiles, "Select visible files", "Selection",
			common.Hotkeys.FilePanelSelectFiles),
		paletteItem(commandIDSelectDirectories, "Select visible folders", "Selection",
			common.Hotkeys.FilePanelSelectDirectories),
		paletteItem(commandIDSort, "Choose sort order", "View", common.Hotkeys.OpenSortOptionsMenu),
		paletteItem(commandIDReverseSort, "Reverse sort order", "View", common.Hotkeys.ToggleReverseSort),
		paletteItem(commandIDCreate, "Create file or folder", "Files", common.Hotkeys.FilePanelItemCreate),
		paletteItem(commandIDRename, "Rename focused item", "Files", common.Hotkeys.FilePanelItemRename),
		paletteItem(commandIDBatchRenamePrompt,
			"Preview batch rename in Xelif prompt", "Files", common.Hotkeys.OpenSPFPrompt,
			"replace", "prefix", "suffix"),
		paletteItem(commandIDBatchRenameApply, "Apply previewed batch rename", "Files", nil, "confirm"),
		paletteItem(commandIDBatchRenameCancel, "Cancel previewed batch rename", "Files", nil),
		paletteItem(commandIDCopy, "Copy", "Files", common.Hotkeys.CopyItems),
		paletteItem(commandIDCut, "Cut", "Files", common.Hotkeys.CutItems),
		paletteItem(commandIDPaste, "Paste", "Files", common.Hotkeys.PasteItems),
		paletteItem(commandIDUndo, "Undo last rename or move", "Files", common.Hotkeys.UndoLastFileOperation),
		paletteItem(commandIDTrash, "Move to trash", "Files", common.Hotkeys.DeleteItems),
		paletteItem(commandIDDelete, "Delete permanently", "Files", common.Hotkeys.PermanentlyDeleteItems),
		paletteItem(commandIDCopyPath, "Copy path", "Clipboard", common.Hotkeys.CopyPath),
		paletteItem(commandIDCopyDirectory, "Copy current directory", "Clipboard", common.Hotkeys.CopyPWD),
		paletteItem(commandIDFavorite, "Toggle favourite folder", "Navigation", common.Hotkeys.PinnedDirectory,
			"pin", "favorite"),
		paletteItem(commandIDCompress, "Compress selection", "Archives", common.Hotkeys.CompressFile, "zip"),
		paletteItem(commandIDExtract, "Extract archive", "Archives", common.Hotkeys.ExtractFile, "unzip"),
		paletteItem(commandIDBrowseArchive, "Browse ZIP as a folder", "Archives", nil, "open", "preview"),
		paletteItem(commandIDBrowseRemote, "Browse rclone remotes", "Remote", nil, "cloud", "download"),
		paletteItem(commandIDGitDiff, "Show Git diff", "Git", nil, "changes", "patch"),
		paletteItem(commandIDGitStagedDiff, "Show staged Git diff", "Git", nil, "index", "patch"),
		paletteItem(commandIDGitStage, "Stage selected items", "Git", nil, "add", "index"),
		paletteItem(commandIDGitUnstage, "Unstage selected items", "Git", nil, "reset", "index"),
		paletteItem(commandIDGitDiscard, "Discard tracked changes", "Git", nil, "restore", "revert"),
		paletteItem(commandIDFindDuplicates, "Find duplicate files here", "Tools", nil, "sha256", "identical"),
		paletteItem(commandIDCancelDuplicates, "Cancel duplicate scan", "Tools", nil, "stop"),
		paletteItem(commandIDCompareDirectories, "Compare focused and next folders", "Tools", nil, "diff"),
		paletteItem(commandIDCompareContent, "Compare folders by content", "Tools", nil, "sha256"),
		paletteItem(commandIDPreviewCopyMissing, "Preview copy missing: focused → next", "Tools", nil, "sync"),
		paletteItem(commandIDApplyCopyMissing, "Apply previewed copy-missing", "Tools", nil, "sync", "confirm"),
		paletteItem(commandIDCancelCompare, "Cancel folder comparison", "Tools", nil, "stop"),
		paletteItem(commandIDMetadataPrompt, "Preview metadata edit in Xelif prompt", "Metadata",
			common.Hotkeys.OpenSPFPrompt, "timestamp", "permissions"),
		paletteItem(commandIDMetadataApply, "Apply previewed metadata edit", "Metadata", nil, "confirm"),
		paletteItem(commandIDMetadataRestore, "Restore last metadata edit", "Metadata", nil, "undo", "journal"),
		paletteItem(commandIDOpenEditor, "Open focused item in editor", "Open With",
			common.Hotkeys.OpenFileWithEditor),
		paletteItem(commandIDOpenDirectoryEditor, "Open folder in editor", "Open With",
			common.Hotkeys.OpenCurrentDirectoryWithEditor),
		paletteItem(commandIDCommandLine, "Open shell prompt", "Terminal", common.Hotkeys.OpenCommandLine),
		paletteItem(commandIDSPFPrompt, "Open Xelif prompt", "Terminal", common.Hotkeys.OpenSPFPrompt),
		paletteItem(commandIDWorkspacePrompt, "Manage named workspaces", "Workspaces",
			common.Hotkeys.OpenSPFPrompt, "session", "layout", "save"),
		paletteItem(commandIDSavedSearchPrompt, "Manage saved searches", "Search",
			common.Hotkeys.OpenSPFPrompt, "query", "filter", "save"),
		paletteItem(commandIDCatalogPrompt, "Manage tags", "Organize",
			common.Hotkeys.OpenSPFPrompt, "label", "virtual folder"),
		paletteItem(commandIDCollectionPrompt, "Manage collections", "Organize", nil, "save", "virtual folder"),
		paletteItem(commandIDHelp, "Open help", "General", common.Hotkeys.OpenHelpMenu),
		paletteItem(commandIDFocusSidebar, "Focus sidebar", "Focus", common.Hotkeys.FocusOnSidebar),
		paletteItem(commandIDFocusProcesses, "Focus processes", "Focus", common.Hotkeys.FocusOnProcessBar),
		paletteItem(commandIDFocusMetadata, "Focus metadata", "Focus", common.Hotkeys.FocusOnMetaData),
		paletteItem(commandIDQuit, "Quit Xelif", "General", common.Hotkeys.Quit),
	}
	items = m.appendStoredPaletteItems(items)
	items = append(items, m.pluginCommandPaletteItems(hasOperationItems)...)
	m.updatePaletteAvailability(items, panel, hasFocusedItem, hasOperationItems)
	return items
}

func (m *model) updatePaletteAvailability(items []commandpalette.Item, panel *filepanel.Model,
	hasFocusedItem, hasOperationItems bool) {
	for index := range items {
		switch items[index].ID {
		case commandIDOpen, commandIDOpenEditor, commandIDExtract:
			disablePaletteItem(&items[index], !hasFocusedItem, "no focused item")
		case commandIDBrowseArchive:
			disablePaletteItem(&items[index], !hasFocusedItem ||
				!archivebrowser.Supported(panel.GetFocusedItem().Location), "focused item is not a ZIP archive")
		case commandIDRename:
			disablePaletteItem(&items[index], !hasFocusedItem || panel.PanelMode == filepanel.SelectMode,
				"rename requires one item in browser mode")
		case commandIDBatchRenamePrompt:
			disablePaletteItem(&items[index], !hasOperationItems, "no selected item")
		case commandIDBatchRenameApply, commandIDBatchRenameCancel:
			disablePaletteItem(&items[index], m.pendingBatchRename == nil, "no previewed batch rename")
		case commandIDCopy, commandIDCut, commandIDTrash, commandIDDelete, commandIDCopyPath,
			commandIDCompress:
			disablePaletteItem(&items[index], !hasOperationItems, "no selected item")
		case commandIDPaste:
			disablePaletteItem(&items[index], m.clipboard.Len() == 0, "clipboard is empty")
		case commandIDUndo:
			disablePaletteItem(&items[index], m.lastUndo == nil, "nothing to undo")
		case commandIDClosePanel:
			disablePaletteItem(&items[index], m.fileModel.PanelCount() <= 1,
				"at least one panel must remain")
		case commandIDCloseTab:
			disablePaletteItem(&items[index], panel.TabCount() <= 1, "at least one tab must remain")
		case commandIDZoxide:
			disablePaletteItem(&items[index], m.zClient == nil, "zoxide is unavailable")
		case commandIDFocusProcesses, commandIDFocusMetadata:
			disablePaletteItem(&items[index], !m.toggleFooter, "footer is hidden")
		case commandIDSelectAll, commandIDSelectRange, commandIDInvertSelection,
			commandIDSelectFiles, commandIDSelectDirectories:
			disablePaletteItem(&items[index], panel.PanelMode != filepanel.SelectMode,
				"selection mode is inactive")
		case commandIDGitStage, commandIDGitUnstage, commandIDGitDiscard:
			disablePaletteItem(&items[index], !hasOperationItems, "no selected item")
		case commandIDCancelDuplicates:
			disablePaletteItem(&items[index], m.duplicateScanCancel == nil, "no duplicate scan is running")
		case commandIDCompareDirectories, commandIDCompareContent, commandIDPreviewCopyMissing:
			disablePaletteItem(&items[index], m.fileModel.PanelCount() < minDirectoryComparePanels,
				"open at least two panels")
		case commandIDApplyCopyMissing:
			disablePaletteItem(&items[index], m.pendingSync == nil, "no copy-missing preview is pending")
		case commandIDCancelCompare:
			disablePaletteItem(&items[index], m.dirCompareCancel == nil, "no folder comparison is running")
		case commandIDMetadataPrompt:
			disablePaletteItem(&items[index], !hasOperationItems, "no selected item")
		case commandIDMetadataApply:
			disablePaletteItem(&items[index], m.pendingMetadataPatch == nil, "no metadata preview is pending")
		case commandIDMetadataRestore:
			disablePaletteItem(&items[index], m.lastMetadataJournal == "", "no metadata edit is available to restore")
		}
	}
}

func (m *model) appendStoredPaletteItems(items []commandpalette.Item) []commandpalette.Item {
	if m.sessionStore != nil {
		names, err := m.sessionStore.ListWorkspaces()
		if err != nil {
			item := paletteItem(commandIDWorkspaceLoadPrefix, "Saved workspaces unavailable", "Workspaces", nil)
			disablePaletteItem(&item, true, err.Error())
			items = append(items, item)
		} else {
			for _, name := range names {
				items = append(items, paletteItem(commandIDWorkspaceLoadPrefix+name,
					"Load workspace: "+name, "Workspaces", nil, "session", "layout"))
			}
		}
	}
	if m.savedSearchStore != nil {
		searches, err := m.savedSearchStore.List()
		if err != nil {
			item := paletteItem(commandIDSavedSearchPrefix, "Saved searches unavailable", "Search", nil)
			disablePaletteItem(&item, true, err.Error())
			items = append(items, item)
		} else {
			for _, search := range searches {
				items = append(items, paletteItem(commandIDSavedSearchPrefix+search.Name,
					"Run saved search: "+search.Name, "Search", nil, search.Query, "filter"))
			}
		}
	}
	if m.catalogModel != nil {
		for _, tag := range m.catalogModel.ListTags() {
			items = append(items, paletteItem(commandIDTagFindPrefix+tag,
				"Find tag: "+tag, "Organize", nil, "label", "collection"))
		}
		for _, name := range m.catalogModel.ListCollections() {
			items = append(items, paletteItem(commandIDCollectionPrefix+name,
				"Open collection: "+name, "Organize", nil, "virtual folder"))
		}
	}
	return items
}

func paletteItem(id, label, category string, hotkeys []string, keywords ...string) commandpalette.Item {
	return commandpalette.Item{
		ID: id, Label: label, Category: category, Hotkeys: hotkeys, Keywords: keywords,
	}
}

func disablePaletteItem(item *commandpalette.Item, disabled bool, reason string) {
	if !disabled {
		return
	}
	item.Disabled = true
	item.DisabledReason = reason
}

func (m *model) applyCommandPaletteAction(action common.ModelAction) tea.Cmd {
	execute, ok := action.(commandpalette.ExecuteAction)
	if !ok {
		return nil
	}
	if cmd, handled := m.applyPaletteLaunchCommand(execute.ID); handled {
		return cmd
	}
	if cmd, handled := m.applyStoredPaletteCommand(execute.ID); handled {
		return cmd
	}
	if cmd, handled := m.applyGitPaletteCommand(execute.ID); handled {
		return cmd
	}
	if cmd, handled := m.applyMetadataPaletteCommand(execute.ID); handled {
		return cmd
	}
	switch execute.ID {
	case commandIDBatchRenameApply:
		_, cmd, err := m.executeBatchRenameAction(common.BatchRenameAction{Operation: common.BatchRenameApply})
		if err != nil {
			m.showUserError("Could not apply batch rename", err)
		}
		return cmd
	case commandIDBatchRenameCancel:
		_, _, err := m.executeBatchRenameAction(common.BatchRenameAction{Operation: common.BatchRenameCancel})
		if err != nil {
			m.showUserError("Could not cancel batch rename", err)
		}
		return nil
	case commandIDBrowseArchive:
		if err := m.openArchive(m.getFocusedFilePanel().GetFocusedItem().Location); err != nil {
			m.showUserError("Could not browse archive", err)
		}
		return nil
	case commandIDBrowseRemote:
		return m.openRemoteBrowserCmd()
	case commandIDFindDuplicates:
		return m.startDuplicateScan(m.getFocusedFilePanel().Location)
	case commandIDCancelDuplicates:
		m.cancelDuplicateScan()
		return nil
	case commandIDCompareDirectories:
		return m.startDirectoryCompare(false)
	case commandIDCompareContent:
		return m.startDirectoryCompare(true)
	case commandIDPreviewCopyMissing:
		return m.previewDirectorySync()
	case commandIDApplyCopyMissing:
		return m.applyPendingDirectorySync()
	case commandIDCancelCompare:
		m.cancelDirectoryCompare()
		return nil
	}
	return m.runPaletteHotkey(execute.ID)
}

func (m *model) applyPaletteLaunchCommand(id string) (tea.Cmd, bool) {
	switch id {
	case commandIDOperations:
		m.openOperations()
		return nil, true
	case commandIDWorkspacePrompt:
		m.promptModal.OpenCommand(prompt.WorkspaceCommand + " ")
		return nil, true
	case commandIDSavedSearchPrompt:
		m.promptModal.OpenCommand(prompt.SearchCommand + " ")
		return nil, true
	case commandIDBatchRenamePrompt:
		m.promptModal.OpenCommand(prompt.BatchRenameCommand + " ")
		return nil, true
	case commandIDCatalogPrompt:
		m.promptModal.OpenCommand(prompt.TagCommand + " ")
		return nil, true
	case commandIDCollectionPrompt:
		m.promptModal.OpenCommand(prompt.CollectionCommand + " ")
		return nil, true
	case commandIDMetadataPrompt:
		m.promptModal.OpenCommand(prompt.MetadataCommand + " ")
		return nil, true
	case commandIDOpen:
		m.enterPanel()
		return nil, true
	case commandIDQuit:
		return m.handleKeyInput(utils.TeaRuneKeyMsg(common.Hotkeys.Quit[0])), true
	case commandIDPluginRefresh:
		m.refreshPluginCommandsFromPalette()
		return nil, true
	}
	if strings.HasPrefix(id, commandIDPluginRunPrefix) {
		return m.runPluginCommand(strings.TrimPrefix(id, commandIDPluginRunPrefix)), true
	}
	return nil, false
}

func (m *model) applyGitPaletteCommand(id string) (tea.Cmd, bool) {
	switch id {
	case commandIDGitDiff:
		return m.gitDiffCmd(false, m.gitOperationPaths()), true
	case commandIDGitStagedDiff:
		return m.gitDiffCmd(true, m.gitOperationPaths()), true
	case commandIDGitStage:
		return m.gitMutationCmd(gitMutationStage, m.gitOperationPaths()), true
	case commandIDGitUnstage:
		return m.gitMutationCmd(gitMutationUnstage, m.gitOperationPaths()), true
	case commandIDGitDiscard:
		return m.confirmGitDiscard(m.gitOperationPaths()), true
	default:
		return nil, false
	}
}

func (m *model) applyMetadataPaletteCommand(id string) (tea.Cmd, bool) {
	switch id {
	case commandIDMetadataApply:
		_, cmd, err := m.executeMetadataAction(common.MetadataAction{Operation: common.MetadataApply})
		if err != nil {
			m.showUserError("Could not apply metadata", err)
		}
		return cmd, true
	case commandIDMetadataRestore:
		_, cmd, err := m.executeMetadataAction(common.MetadataAction{Operation: common.MetadataRestore})
		if err != nil {
			m.showUserError("Could not restore metadata", err)
		}
		return cmd, true
	default:
		return nil, false
	}
}

func (m *model) runPaletteHotkey(id string) tea.Cmd {
	hotkeys := m.commandHotkeys(id)
	if len(hotkeys) == 0 {
		m.showUserError("Could not run command", errors.New("unknown command: "+id))
		return nil
	}
	return m.mainKey(hotkeys[0])
}

func (m *model) applyStoredPaletteCommand(id string) (tea.Cmd, bool) {
	if strings.HasPrefix(id, commandIDWorkspaceLoadPrefix) {
		name := strings.TrimPrefix(id, commandIDWorkspaceLoadPrefix)
		if err := m.loadWorkspace(name); err != nil {
			m.showUserError("Could not load workspace", err)
			return nil, true
		}
		return m.updateComponentDimensions(), true
	}
	if strings.HasPrefix(id, commandIDSavedSearchPrefix) {
		name := strings.TrimPrefix(id, commandIDSavedSearchPrefix)
		_, cmd, err := m.executeSavedSearchAction(common.SearchAction{Operation: common.SearchRun, Name: name})
		if err != nil {
			m.showUserError("Could not run saved search", err)
			return nil, true
		}
		return cmd, true
	}
	if strings.HasPrefix(id, commandIDTagFindPrefix) {
		name := strings.TrimPrefix(id, commandIDTagFindPrefix)
		_, _, err := m.executeCatalogAction(common.CatalogAction{Operation: common.TagFind, Name: name})
		if err != nil {
			m.showUserError("Could not find tagged paths", err)
		}
		return nil, true
	}
	if strings.HasPrefix(id, commandIDCollectionPrefix) {
		name := strings.TrimPrefix(id, commandIDCollectionPrefix)
		_, _, err := m.executeCatalogAction(common.CatalogAction{Operation: common.CollectionOpen, Name: name})
		if err != nil {
			m.showUserError("Could not open collection", err)
		}
		return nil, true
	}
	return nil, false
}

func navigationCommandHotkeys(id string) []string {
	switch id {
	case commandIDParent:
		return common.Hotkeys.ParentDirectory
	case commandIDGlobalSearch:
		return common.Hotkeys.OpenGlobalSearch
	case commandIDZoxide:
		return common.Hotkeys.OpenZoxide
	case commandIDNewPanel:
		return common.Hotkeys.CreateNewFilePanel
	case commandIDSplitPanel:
		return common.Hotkeys.SplitFilePanel
	case commandIDClosePanel:
		return common.Hotkeys.CloseFilePanel
	case commandIDNewTab:
		return common.Hotkeys.NewFileTab
	case commandIDCloseTab:
		return common.Hotkeys.CloseFileTab
	case commandIDNextTab:
		return common.Hotkeys.NextFileTab
	case commandIDPreviousTab:
		return common.Hotkeys.PreviousFileTab
	case commandIDHistoryBack:
		return common.Hotkeys.HistoryBack
	case commandIDHistoryForward:
		return common.Hotkeys.HistoryForward
	case commandIDTogglePreview:
		return common.Hotkeys.ToggleFilePreviewPanel
	case commandIDToggleFooter:
		return common.Hotkeys.ToggleFooter
	case commandIDToggleDotFiles:
		return common.Hotkeys.ToggleDotFile
	case commandIDToggleSelection:
		return common.Hotkeys.ChangePanelMode
	case commandIDSelectAll:
		return common.Hotkeys.FilePanelSelectAllItem
	case commandIDSelectRange:
		return common.Hotkeys.FilePanelSelectRange
	case commandIDInvertSelection:
		return common.Hotkeys.FilePanelInvertSelection
	case commandIDSelectFiles:
		return common.Hotkeys.FilePanelSelectFiles
	case commandIDSelectDirectories:
		return common.Hotkeys.FilePanelSelectDirectories
	case commandIDSort:
		return common.Hotkeys.OpenSortOptionsMenu
	case commandIDReverseSort:
		return common.Hotkeys.ToggleReverseSort
	default:
		return nil
	}
}

func (m *model) commandHotkeys(id string) []string {
	if hotkeys := navigationCommandHotkeys(id); hotkeys != nil {
		return hotkeys
	}
	switch id {
	case commandIDCreate:
		return common.Hotkeys.FilePanelItemCreate
	case commandIDRename:
		return common.Hotkeys.FilePanelItemRename
	case commandIDCopy:
		return common.Hotkeys.CopyItems
	case commandIDCut:
		return common.Hotkeys.CutItems
	case commandIDPaste:
		return common.Hotkeys.PasteItems
	case commandIDUndo:
		return common.Hotkeys.UndoLastFileOperation
	case commandIDTrash:
		return common.Hotkeys.DeleteItems
	case commandIDDelete:
		return common.Hotkeys.PermanentlyDeleteItems
	case commandIDCopyPath:
		return common.Hotkeys.CopyPath
	case commandIDCopyDirectory:
		return common.Hotkeys.CopyPWD
	case commandIDFavorite:
		return common.Hotkeys.PinnedDirectory
	case commandIDCompress:
		return common.Hotkeys.CompressFile
	case commandIDExtract:
		return common.Hotkeys.ExtractFile
	case commandIDOpenEditor:
		return common.Hotkeys.OpenFileWithEditor
	case commandIDOpenDirectoryEditor:
		return common.Hotkeys.OpenCurrentDirectoryWithEditor
	case commandIDCommandLine:
		return common.Hotkeys.OpenCommandLine
	case commandIDSPFPrompt, commandIDWorkspacePrompt, commandIDSavedSearchPrompt,
		commandIDBatchRenamePrompt, commandIDCatalogPrompt, commandIDMetadataPrompt:
		return common.Hotkeys.OpenSPFPrompt
	case commandIDHelp:
		return common.Hotkeys.OpenHelpMenu
	case commandIDFocusSidebar:
		return common.Hotkeys.FocusOnSidebar
	case commandIDFocusProcesses:
		return common.Hotkeys.FocusOnProcessBar
	case commandIDFocusMetadata:
		return common.Hotkeys.FocusOnMetaData
	default:
		return nil
	}
}

func (m *model) setCommandPaletteSize() {
	m.commandPalette.SetMaxHeight(m.fullHeight / 2) //nolint:mnd // Palette uses half-height modal.
	m.commandPalette.SetWidth(2 * m.fullWidth / 3)  //nolint:mnd // Labels and hotkeys need horizontal room.
}

func operationsPaletteItem() commandpalette.Item {
	return paletteItem(
		commandIDOperations,
		"Operations",
		"View",
		common.Hotkeys.ToggleOperations,
		"tasks",
		"background",
		"cancel",
		"progress",
	)
}
