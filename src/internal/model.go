package internal

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/yorukot/superfile/src/config/icon"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/pkg/utils"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/barasher/go-exiftool"

	"github.com/yorukot/superfile/src/internal/ui/filepanel"
	"github.com/yorukot/superfile/src/internal/ui/metadata"
	"github.com/yorukot/superfile/src/internal/ui/notify"
	"github.com/yorukot/superfile/src/internal/ui/preview"
	"github.com/yorukot/superfile/src/internal/ui/processbar"

	variable "github.com/yorukot/superfile/src/config"
	zoxideui "github.com/yorukot/superfile/src/internal/ui/zoxide"
	stringfunction "github.com/yorukot/superfile/src/pkg/string_function"
)

// These represent model's state information, its not a global preperty
var (
	LastTimeCursorMove = [2]int{int(time.Now().UnixMicro()), 0} //nolint: gochecknoglobals // TODO: Move to model struct
	et                 *exiftool.Exiftool                       //nolint: gochecknoglobals // TODO: Move to model struct
)

// Initialize and return model with default configs
// It returns only tea.Model because when it used in main, the return value
// is passed to tea.NewProgram() which accepts tea.Model
// Either way type 'model' is not exported, so there is not way main package can
// be aware of it, and use it directly
func InitialModel(firstPanelPaths []string, firstUseCheck, explicitPaths bool) tea.Model {
	toggleDotFile, toggleFooter, zClient := initialConfig(firstPanelPaths)
	m := defaultModelConfig(toggleDotFile, toggleFooter, firstUseCheck, firstPanelPaths, zClient)
	m.initializePluginCommands()
	if !explicitPaths && common.Config.RestoreLastSession {
		if err := m.restoreLastSession(); err != nil {
			slog.Error("Could not restore the previous session", "error", err)
		}
	}
	if err := m.startFolderWatcher(); err != nil {
		slog.Warn("Filesystem notifications are unavailable; using polling", "error", err)
	}
	m.restoreOperationsVisibility(variable.OperationCenterVisibility)
	return m
}

// Init function to be called by Bubble tea framework, sets windows title,
// cursos blinking and starts message streamming channel
// Note : What init should do, for example read file panel data, read sidebar directories, and
// disk, is being done in at the creation of model of object. Right now creation of model object
// and its initialization isn't well separated.
func (m *model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink, // Assuming textinput.Blink is a valid command
		processCmdToTeaCmd(m.processBarModel.GetListenCmd()),
		m.syncFolderWatcherCmd(),
		m.waitFolderWatchEventCmd(),
		m.waitFolderWatchErrorCmd(),
		m.refreshGitStatusCmd(true),
		refreshTickCmd(),
	)
}

// Update function for bubble tea to provide internal communication to the
// application
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	slog.Debug("model.Update() called", "msgType", reflect.TypeOf(msg))
	operationsWasOpen, globalSearchWasOpen, operationsShortcut := m.operations.open, m.globalSearchModal.IsOpen(), m.handleOperationsShortcut(
		msg,
	)

	var sidebarCmd, inputCmd, updateCmd, panelCmd,
		metadataCmd, filePreviewCmd, helpMenuCmd, resizeCmd, mouseCmd, globalSearchCmd,
		commandPaletteCmd, folderWatchCmd, gitStatusCmd tea.Cmd

	// These are above the key message handing to prevent issues with firstKeyInput
	// if someone presses `/` to focus to searchBar, searchBar will otherwise
	// get `/` input too.
	sidebarCmd = m.sidebarModel.UpdateState(msg)
	// Necessary for blinking. Can't do this in HandleKey, as we only pass KeyMsg there
	helpMenuCmd = m.helpMenu.HandleTeaMsg(msg)
	if globalSearchWasOpen && !operationsShortcut && !m.operationsBlockPickerInput(msg) {
		action, cmd := m.globalSearchModal.HandleUpdate(msg)
		globalSearchCmd = cmd
		m.applyGlobalSearchAction(action)
	}
	commandPaletteWasOpen := m.commandPalette.IsOpen()
	if commandPaletteWasOpen && !globalSearchWasOpen && !operationsShortcut && !m.operationsBlockPickerInput(msg) {
		action, cmd := m.commandPalette.HandleUpdate(msg)
		commandPaletteCmd = tea.Batch(cmd, m.applyCommandPaletteAction(action))
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		resizeCmd = m.handleWindowResize(msg)
	case tea.MouseMsg:
		mouseCmd = m.handleMouseMsg(msg)
	case tea.KeyPressMsg:
		if !operationsShortcut && ((!globalSearchWasOpen && !commandPaletteWasOpen) || m.operations.open) {
			inputCmd = m.handleKeyInput(msg)
		}
	case tea.PasteMsg:
		m.acceptPasteUnlessOperations(msg.Content)

	// Has to handle zoxide messages separately as they could be generated via
	// zoxide update commands, or batched commands from textinput
	// Cannot do it like processbar messages
	case zoxideui.UpdateMsg:
		slog.Debug("Got ModelUpdate message", "id", msg.GetReqID())
		updateCmd = msg.Apply(&m.zoxideModal)

	// Its a pain to interconvert commands like processBar
	case preview.UpdateMsg:
		slog.Debug("Got ModelUpdate message", "id", msg.GetReqID())
		updateCmd = m.fileModel.UpdatePreviewPanel(msg)
	case refreshTickMsg:
		updateCmd = refreshTickCmd()
	case folderWatchEventMsg, folderWatchErrorMsg, folderWatchSyncMsg:
		folderWatchCmd, gitStatusCmd = m.handleFolderWatchMessage(msg)
	case gitOperationMsg:
		gitStatusCmd = m.applyGitOperation(msg)
	case editorFinishedMsg, gitStatusUpdateMsg, batchRenameOperationMsg, archiveOperationMsg,
		remoteOperationMsg, pluginCommandFinishedMsg, duplicateScanMsg, directoryCompareMsg,
		directorySyncMsg, metadataOperationMsg:
		m.applyOperationResult(msg)
	case ModelUpdateMessage:
		slog.Debug("Got ModelUpdate message", "id", msg.GetReqID())
		updateCmd = msg.ApplyToModel(m)

	default:
		slog.Debug("Message of type that is not explicitly handled")
	}
	m.refreshPanelsAfterOperation(msg)

	// This is needed for blink, etc to work
	panelCmd = m.updateComponentState(
		componentMessageAfterPicker(
			msg,
			globalSearchWasOpen || commandPaletteWasOpen || operationsWasOpen || operationsShortcut,
		),
	)

	m.updateModelStateForMessage(msg)
	folderWatchCmd = tea.Batch(folderWatchCmd, m.syncFolderWatcherCmd())
	gitStatusCmd = tea.Batch(gitStatusCmd, m.refreshGitStatusCmd(false))
	filePreviewCmd = m.fileModel.GetFilePreviewCmd(false)

	metadataCmd = m.getMetadataCmd()

	return m, tea.Batch(sidebarCmd, helpMenuCmd, inputCmd, updateCmd,
		panelCmd, metadataCmd, filePreviewCmd, resizeCmd, mouseCmd, globalSearchCmd, commandPaletteCmd,
		folderWatchCmd, gitStatusCmd)
}

func (m *model) acceptPasteUnlessOperations(content string) {
	if !m.operations.open {
		m.acceptDroppedPaths(content)
		return
	}
	if m.operations.details {
		return
	}
	content = strings.ReplaceAll(content, "\n", " ")
	m.operations.filter.SetValue(m.operations.filter.Value() + common.MakePrintableWithEscCheck(content, false))
	m.operations.selected = ""
	m.operations.cursor, m.operations.offset = 0, 0
}

func componentMessageAfterPicker(msg tea.Msg, pickerWasOpen bool) tea.Msg {
	if _, key := msg.(tea.KeyPressMsg); key && pickerWasOpen {
		// A consumed key must not also submit a dialog the picker just opened.
		// A nil update still clears the component's first-input guards.
		return nil
	}
	return msg
}

func (m *model) handleMouseMsg(msg tea.MouseMsg) tea.Cmd {
	if m.operations.open && !m.spfError.IsOpen() {
		m.handleOperationMouse(msg)
		return nil
	}
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		return m.handleMouseWheel(msg)
	case tea.MouseClickMsg:
		return m.handleMouseClick(msg)
	}
	return nil
}

func (m *model) handleMouseWheel(msg tea.MouseWheelMsg) tea.Cmd {
	if m.handleOverlayMouseWheel(msg) {
		return nil
	}
	if m.spfError.IsOpen() || m.IsOverlayModelOpen() || m.contextMenu.open {
		return nil
	}
	if m.handleTabWheel(msg.Mouse()) {
		return nil
	}
	if !m.sidebarModel.Disabled() && msg.X >= 0 && msg.X < m.sidebarModel.GetWidth() &&
		msg.Y >= 0 && msg.Y < m.sidebarModel.GetHeight() {
		m.focusPanel = sidebarFocus
		m.sidebarModel.SearchBarBlur()
	} else if msg.Y >= 0 && msg.Y < m.fileModel.Height {
		start := m.sidebarModel.GetWidth()
		for index := range m.fileModel.FilePanels {
			end := start + m.fileModel.FilePanels[index].GetWidth()
			if msg.X > start && msg.X < end-1 {
				m.fileModel.FocusPanel(index)
				m.focusPanel = nonePanelFocus
				m.sidebarModel.SearchBarBlur()
				break
			}
			start = end
		}
	}
	wheelMainAction(msg.Button, m)
	return nil
}

func (m *model) handleMouseClick(msg tea.MouseClickMsg) tea.Cmd {
	if cmd, handled := m.handleModalMouseClick(msg.Mouse()); handled {
		return cmd
	}
	if cmd, handled := m.handleViewerMouseClick(msg); handled {
		return cmd
	}
	if msg.Button == tea.MouseLeft {
		if cmd, handled := m.contextMenuClick(msg.Mouse()); handled {
			return cmd
		}
		return m.handleLeftMouseClick(msg.Mouse())
	}
	if msg.Button == tea.MouseRight {
		return m.handleRightMouseClick(msg.Mouse())
	}
	if !m.IsOverlayModelOpen() && !m.contextMenu.open && !m.spfError.IsOpen() &&
		!m.fileModel.Renaming && !m.sidebarModel.IsRenaming() {
		switch msg.Button {
		case tea.MouseBackward:
			m.navigateMouseHistory(false)
		case tea.MouseForward:
			m.navigateMouseHistory(true)
		}
	}
	return nil
}

func (m *model) handleGlobalSearchClick(mouse tea.Mouse) {
	if !insideCentered(mouse, m.fullWidth, m.fullHeight,
		m.globalSearchModal.GetWidth(), m.globalSearchModal.GetMaxHeight()) {
		m.globalSearchModal.Close()
		return
	}
	overlayX := m.fullWidth/common.CenterDivisor - m.globalSearchModal.GetWidth()/common.CenterDivisor
	overlayY := m.fullHeight/common.CenterDivisor - m.globalSearchModal.GetMaxHeight()/common.CenterDivisor
	if mouse.X >= overlayX && mouse.X < overlayX+m.globalSearchModal.GetWidth() {
		m.applyGlobalSearchAction(m.globalSearchModal.HandleClick(mouse.Y - overlayY))
	}
}

func (m *model) handleCommandPaletteClick(mouse tea.Mouse) tea.Cmd {
	if !insideCentered(mouse, m.fullWidth, m.fullHeight,
		m.commandPalette.GetWidth(), m.commandPalette.GetMaxHeight()) {
		m.commandPalette.Close()
		return nil
	}
	overlayX := m.fullWidth/common.CenterDivisor - m.commandPalette.GetWidth()/common.CenterDivisor
	overlayY := m.fullHeight/common.CenterDivisor - m.commandPalette.GetMaxHeight()/common.CenterDivisor
	if mouse.X < overlayX || mouse.X >= overlayX+m.commandPalette.GetWidth() {
		return nil
	}
	return m.applyCommandPaletteAction(m.commandPalette.HandleClick(mouse.Y - overlayY))
}

func (m *model) handleRightMouseClick(mouse tea.Mouse) tea.Cmd {
	m.mouse = mouseState{}
	for i := range m.fileModel.FilePanels {
		m.fileModel.FilePanels[i].SearchBar.Blur()
	}
	if m.spfError.IsOpen() || m.IsOverlayModelOpen() || m.fileModel.Renaming ||
		m.sidebarModel.IsRenaming() || !m.firstLoadingComplete ||
		mouse.Y < 0 || mouse.Y >= m.fileModel.Height {
		return nil
	}

	if !m.sidebarModel.Disabled() && mouse.X > 0 && mouse.X < m.sidebarModel.GetWidth()-1 {
		if path, ok := m.sidebarModel.FocusDirectoryAtY(mouse.Y, m.focusPanel == sidebarFocus); ok {
			m.sidebarModel.SearchBarBlur()
			m.focusPanel = sidebarFocus
			m.getFocusedFilePanel().IsFocused = false
			m.contextMenu = contextMenuState{open: true, targetPath: path, sidebarTarget: true}
			m.positionContextMenu(mouse)
		}
		return nil
	}
	panelX := m.sidebarModel.GetWidth()
	for panelIndex := range m.fileModel.FilePanels {
		panel := &m.fileModel.FilePanels[panelIndex]
		if mouse.X < panelX || mouse.X >= panelX+panel.GetWidth() {
			panelX += panel.GetWidth()
			continue
		}
		localX := mouse.X - panelX
		if localX <= 0 || localX >= panel.GetWidth()-1 {
			return nil
		}
		m.sidebarModel.SearchBarBlur()
		rangeStartPath := panel.GetFocusedItem().Location
		index, hit := panel.EntryIndexAtY(mouse.Y)
		if !hit ||
			!panel.FocusIndex(index) || !m.fileModel.FocusPanel(panelIndex) {
			return nil
		}
		m.focusPanel = nonePanelFocus
		if panel.PanelMode == filepanel.SelectMode && !panel.CheckSelected(panel.GetFocusedItem().Location) {
			panel.ResetSelected()
			panel.SetSelected(panel.GetFocusedItem().Location)
		}
		m.openContextMenu(mouse, panelIndex, panel.GetFocusedItem().Location)
		m.contextMenu.rangeStartPath = rangeStartPath
		return nil
	}
	return nil
}

func (m *model) handleLeftMouseClick(mouse tea.Mouse) tea.Cmd {
	if m.spfError.IsOpen() || m.IsOverlayModelOpen() || m.fileModel.Renaming ||
		m.sidebarModel.IsRenaming() || !m.firstLoadingComplete ||
		m.fullHeight < common.MinimumHeight || m.fullWidth < common.MinimumWidth {
		return nil
	}
	if m.toggleFooter && mouse.Y >= m.fileModel.Height && mouse.Y < m.fullHeight &&
		mouse.X >= 0 && mouse.X < m.processBarModel.GetWidth() {
		m.openOperations()
		return nil
	}
	if mouse.Y < 0 || mouse.Y >= m.fileModel.Height {
		return nil
	}
	if !m.sidebarModel.Disabled() && mouse.X >= 0 && mouse.X < m.sidebarModel.GetWidth() {
		m.handleSidebarLeftClick(mouse)
		return nil
	}

	panelX := m.sidebarModel.GetWidth()
	for panelIndex := range m.fileModel.FilePanels {
		panelWidth := m.fileModel.FilePanels[panelIndex].GetWidth()
		if mouse.X < panelX || mouse.X >= panelX+panelWidth {
			panelX += panelWidth
			continue
		}
		return m.handlePanelLeftClick(mouse, panelIndex, mouse.X-panelX, panelWidth)
	}
	m.mouse = mouseState{}
	return nil
}

func (m *model) handleSidebarLeftClick(mouse tea.Mouse) {
	m.mouse = mouseState{}
	for i := range m.fileModel.FilePanels {
		m.fileModel.FilePanels[i].SearchBar.Blur()
	}
	sidebarFocused := m.focusPanel == sidebarFocus
	m.focusPanel = sidebarFocus
	m.getFocusedFilePanel().IsFocused = false
	if m.sidebarModel.IsSearchRowY(mouse.Y, sidebarFocused) {
		m.sidebarModel.SearchBarFocus()
		m.firstTextInput = true
		return
	}
	if path, ok := m.sidebarModel.FocusDirectoryAtY(mouse.Y, sidebarFocused); ok {
		if err := m.updateCurrentFilePanelDir(path); err != nil {
			m.showUserError("Could not open folder", err)
		}
	}
}

func (m *model) handlePanelLeftClick(mouse tea.Mouse, panelIndex, localX, panelWidth int) tea.Cmd {
	if localX <= 0 || localX >= panelWidth-1 {
		m.mouse = mouseState{}
		return nil
	}

	m.sidebarModel.SearchBarBlur()
	if !m.fileModel.FocusPanel(panelIndex) {
		return nil
	}
	m.focusPanel = nonePanelFocus
	panel := m.getFocusedFilePanel()
	for i := range m.fileModel.FilePanels {
		m.fileModel.FilePanels[i].SearchBar.Blur()
	}

	if mouse.Y == 1 {
		m.mouse = mouseState{}
		m.handlePanelHeaderClick(panel, localX-1)
		return nil
	}

	if mouse.Y == panelActionsRow {
		m.mouse = mouseState{}
		if search, commands := panel.SearchActionAtContentX(localX - 1); search {
			return m.globalSearchModal.Open(panel.Location)
		} else if commands {
			return m.openCommandPalette()
		}
		return nil
	}
	if mouse.Y == panelFilterRow {
		m.mouse = mouseState{}
		panel.SearchBar.Focus()
		m.firstTextInput = true
		return nil
	}
	if panel.NeedRenderHeaders() && mouse.Y == 4 {
		m.mouse = mouseState{}
		panel.ApplySortColumnAtContentX(localX - 1)
		return nil
	}
	if mouse.Y == panel.GetHeight()-1 {
		m.mouse = mouseState{}
		if action, ok := panel.FooterActionAtX(localX); ok {
			switch action {
			case filepanel.FooterActionSort:
				m.sortModal.Open(panel.SortKind)
			case filepanel.FooterActionMode:
				panel.ChangeFilePanelMode()
			}
		}
		return nil
	}
	return m.handlePanelEntryClick(mouse, panelIndex, panel)
}

func (m *model) handlePanelEntryClick(mouse tea.Mouse, panelIndex int, panel *filepanel.Model) tea.Cmd {
	index, hit := panel.EntryIndexAtY(mouse.Y)
	if !hit {
		m.mouse = mouseState{}
		return nil
	}
	if mouse.Mod.Contains(tea.ModShift) {
		if panel.SelectRangeTo(index) {
			panel.PanelMode = filepanel.SelectMode
			panel.FocusIndex(index)
		}
		m.mouse = mouseState{}
		return nil
	}
	if !panel.FocusIndex(index) {
		m.mouse = mouseState{}
		return nil
	}
	if mouse.Mod.Contains(tea.ModCtrl) {
		panel.PanelMode = filepanel.SelectMode
		panel.ToggleSelected(panel.GetFocusedItem().Location)
		panel.ClearSelectionAnchor()
		m.mouse = mouseState{}
		return nil
	}

	path := panel.GetFocusedItem().Location
	now := time.Now()
	isDoubleClick := m.mouse.lastPanel == panelIndex && m.mouse.lastPath == path &&
		!m.mouse.lastClick.IsZero() && now.Sub(m.mouse.lastClick) <= 500*time.Millisecond
	if isDoubleClick {
		m.mouse = mouseState{}
		m.enterPanel()
		return nil
	}

	m.mouse = mouseState{lastPanel: panelIndex, lastPath: path, lastClick: now}
	panel.ClearSelectionAnchor()
	return nil
}

func (m *model) updateModelStateAfterMsg() {
	m.sidebarModel.UpdateDirectoriesIfNeeded()
	m.fileModel.UpdateFilePanelsIfNeeded(false)
	// TODO: Move to utility
	if m.focusPanel != metadataFocus {
		m.fileMetaData.ResetRender()
	}
	// TODO: Entirely remove the need of this variable, and handle first loading via Init()
	// Init() should return a basic model object with all IO waiting via a tea.Cmd
	if !m.firstLoadingComplete {
		m.firstLoadingComplete = true
	}
}

// Note : Maybe we should not trigger metadata fetch for updates
// that dont change the currently selected file panel element
// TODO : At least dont trigger metadata fetch when user is scrolling
// through the metadata panel
func (m *model) getMetadataCmd() tea.Cmd {
	if m.disableMetadata || m.modelQuitState == quitDone {
		return nil
	}
	if m.getFocusedFilePanel().EmptyOrInvalid() {
		m.fileMetaData.SetBlank()
		return nil
	}
	selectedItem := m.getFocusedFilePanel().GetFocusedItem()
	metadataFocused := m.focusPanel == metadataFocus
	// Note : This will cause metadata not being refreshed there is any file update events.
	// We can have a cache with TTL or watch filesystem changes to fix this
	if selectedItem.Location == m.fileMetaData.GetMetadataLocation() &&
		metadataFocused == m.fileMetaData.GetMetadataExpectedFocused() {
		return nil
	}

	if metadataFocused {
		m.fileMetaData.DropMetadataIfInCache(selectedItem.Location)
	}

	if m.fileMetaData.UpdateMetadataIfExistsInCache(selectedItem.Location, metadataFocused) {
		return nil
	}

	m.fileMetaData.SetMetadataLocationAndFocused(selectedItem.Location, metadataFocused)

	if m.fileMetaData.IsBlank() {
		m.fileMetaData.SetInfoMsg(icon.InOperation + icon.Space + "Loading metadata...")
	}

	reqCnt := m.nextIoReqCnt()
	if m.metadataCancel != nil {
		m.metadataCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.metadataCancel = cancel
	m.metadataReqID = reqCnt
	// If there are too many metadata fetches, we need to have a cache with path as a key
	// and timeout based eviction
	slog.Debug("Submitting metadata fetch request", "id", reqCnt, "path", selectedItem.Location)
	return m.trackTask("Metadata", selectedItem.Location, "", true, ctx, cancel, func() tea.Msg {
		meta, err := metadata.GetMetadataContext(ctx, selectedItem.Location, metadataFocused, et)
		msg := NewMetadataMsg(meta, metadataFocused, reqCnt)
		msg.err = err
		return msg
	})
}

// Adjust window size based on msg information
func (m *model) handleWindowResize(msg tea.WindowSizeMsg) tea.Cmd {
	m.fullHeight = msg.Height
	m.fullWidth = msg.Width
	m.setHeightValues()
	return m.updateComponentDimensions()
}

func (m *model) setHeightValues() {
	//nolint: gocritic // This is to be separated out to a function, and made better later. No need to refactor here
	if !m.toggleFooter {
		m.footerHeight = 0
	} else if m.fullHeight < common.HeightBreakA {
		m.footerHeight = 6
	} else if m.fullHeight < common.HeightBreakB {
		m.footerHeight = 7
	} else if m.fullHeight < common.HeightBreakC {
		m.footerHeight = 8
	} else if m.fullHeight < common.HeightBreakD {
		m.footerHeight = 9
	} else {
		m.footerHeight = 10
	}
	// TODO : Make it grow even more for bigger screen sizes.
	// TODO : Calculate the value , instead of manually hard coding it.

	// Main panel height = Total terminal height- 2(file panel border) - footer height
	m.mainPanelHeight = m.fullHeight - common.BorderPadding - utils.FullFooterHeight(m.footerHeight, m.toggleFooter)
}

func (m *model) updateComponentDimensions() tea.Cmd {
	m.setHelpMenuSize()
	m.setPromptModelSize()
	m.setZoxideModelSize()
	m.setGlobalSearchModelSize()
	m.setCommandPaletteSize()
	m.setTextViewerSize()
	m.setArchiveViewerSize()
	m.setRemoteViewerSize()
	m.setFooterComponentSize()

	// File preview panel requires explicit height update, unlike sidebar/file panels
	// which receive height as render parameters and update automatically on each frame
	// Force re-render of preview content with new dimensions
	return m.setMainModelDimensions()
}

func (m *model) setMainModelDimensions() tea.Cmd {
	fileModelWidth := m.fullWidth
	if common.Config.SidebarWidth != 0 {
		fileModelWidth -= common.Config.SidebarWidth + common.BorderPadding
	}
	m.sidebarModel.SetHeight(m.mainPanelHeight + common.BorderPadding)
	return m.fileModel.SetDimensions(fileModelWidth, m.mainPanelHeight+common.BorderPadding)
}

// Set help menu size
func (m *model) setHelpMenuSize() {
	height := m.fullHeight - common.BorderPadding
	width := m.fullWidth - common.BorderPadding
	if m.fullHeight > common.HeightBreakB {
		height = 30
	}
	if m.fullWidth > common.ResponsiveWidthThreshold {
		width = 90
	}
	m.helpMenu.SetDimensions(width, height)
}

func (m *model) setPromptModelSize() {
	// Scale prompt model's maxHeight - 33% of total height
	m.promptModal.SetMaxHeight(m.fullHeight / 3) //nolint:mnd // modal uses third height for layout

	// Scale prompt model's maxHeight - 50% of total height
	m.promptModal.SetWidth(m.fullWidth / 2) //nolint:mnd // modal uses half width for layout
}

func (m *model) setZoxideModelSize() {
	// Scale zoxide model's maxHeight - 50% of total height to accommodate scroll indicators
	m.zoxideModal.SetMaxHeight(m.fullHeight / 2) //nolint:mnd // modal uses half height for layout

	// Scale zoxide model's width - 50% of total width
	m.zoxideModal.SetWidth(m.fullWidth / 2) //nolint:mnd // modal uses half width for layout
}

func (m *model) setGlobalSearchModelSize() {
	m.globalSearchModal.SetMaxHeight(m.fullHeight / 2) //nolint:mnd // Search uses half-height modal.
	m.globalSearchModal.SetWidth(2 * m.fullWidth / 3)  //nolint:mnd // Search favors readable full paths.
}

func (m *model) setFooterComponentSize() {
	var width, clipBoardwidth, height int
	height = m.footerHeight + common.BorderPadding
	width = m.fullWidth / utils.CntFooterPanels
	clipBoardwidth = width + m.fullWidth%utils.CntFooterPanels
	m.fileMetaData.SetDimensions(width, height)
	m.processBarModel.SetDimensions(width, height)
	m.clipboard.SetDimensions(clipBoardwidth, height)
}

// Identify the current state of the application m and properly handle the
// msg keybind pressed
func (m *model) handleKeyInput(msg tea.KeyPressMsg) tea.Cmd {
	slog.Debug("model.handleKeyInput", "msg", msg, "typestr", msg.String(),
		"code", msg.Code, "text", msg.Text, "mod", msg.Mod)
	slog.Debug("model.handleKeyInput. model info. ",
		"fileModel.FocusedPanelIndex", m.fileModel.FocusedPanelIndex,
		"filePanel.isFocused", m.getFocusedFilePanel().IsFocused,
		"filePanel.panelMode", m.getFocusedFilePanel().PanelMode,
		"typingModal.open", m.typingModal.open,
		"notifyModel.open", m.notifyModel.IsOpen(),
		"promptModal.open", m.promptModal.IsOpen(),
		"fileModel.renaming", m.fileModel.Renaming,
		"searchBar.focused", m.getFocusedFilePanel().SearchBar.Focused(),
		"helpMenu.open", m.helpMenu.IsOpen(),
		"firstTextInput", m.firstTextInput,
		"focusPanel", m.focusPanel,
	)
	if m.firstUse {
		m.firstUse = false
		return nil
	}
	var cmd tea.Cmd
	cdOnQuit := common.Config.CdOnQuit
	switch {
	case m.spfError.IsOpen():
		cmd = m.spfErrorModelOpenKey(msg.String())
	case m.operations.open:
		cmd = m.handleOperationKey(msg)
	case m.helpMenu.IsOpen():
		m.helpMenu.HandleKey(msg.String())
	case m.promptModal.IsOpen():
		// Ignore keypress. It will be handled in Update call via
		// updateFilePanelState
		// TODO: Convert that to async via tea.Cmd
	case m.zoxideModal.IsOpen():
		// Ignore keypress. It will be handled in Update call via
		// updateFilePanelState
	case m.globalSearchModal.IsOpen():
		// Handled by globalSearchModal.HandleUpdate in Update.
	case m.commandPalette.IsOpen():
		// Handled by commandPalette.HandleUpdate in Update.
	case m.textViewer.IsOpen():
		m.handleTextViewerKey(msg.String())
	case m.archiveViewer.IsOpen():
		cmd = m.handleArchiveViewerKey(msg.String())
	case m.remoteViewer.IsOpen():
		cmd = m.handleRemoteViewerKey(msg.String())
	case m.sortModal.IsOpen():
		m.sortOptionsKey(msg.String())
	case m.typingModal.open:
		cmd = m.typingModalOpenKey(msg.String())

	// Handles all warn models except the warn model for confirming to quit
	case m.notifyModel.IsOpen():
		cmd = m.notifyModelOpenKey(msg.String())
	case m.contextMenu.open:
		cmd = m.contextMenuKey(msg.String())

	// If renaming a object
	case m.fileModel.Renaming:
		m.renamingKey(msg.String())
	case m.sidebarModel.IsRenaming():
		m.sidebarRenamingKey(msg.String())
	// If search bar is open
	case m.getFocusedFilePanel().SearchBar.Focused():
		m.focusOnSearchbarKey(msg.String())
	// If sort options menu is open
	case m.sidebarModel.SearchBarFocused():
		m.sidebarModel.HandleSearchBarKey(msg.String())
	case slices.Contains(common.Hotkeys.Quit, msg.String()):
		m.modelQuitState = quitInitiated

	case slices.Contains(common.Hotkeys.CdQuit, msg.String()):
		m.modelQuitState = quitInitiated
		cdOnQuit = true

	default:
		// Handles general kinds of inputs in the regular state of the application
		cmd = m.mainKey(msg.String())
	}
	return m.finishKeyInput(cmd, cdOnQuit)
}

func (m *model) finishKeyInput(cmd tea.Cmd, cdOnQuit bool) tea.Cmd {
	// If quiting input pressed, check if has any running process and displays a
	// warn. Otherwise just quits application
	if m.modelQuitState == quitInitiated {
		if m.processBarModel.HasRunningProcesses() {
			// Dont quit now, get a confirmation first.
			m.modelQuitState = quitConfirmationInitiated
			m.warnModalForQuit()
			return cmd
		}
		m.modelQuitState = quitConfirmationReceived
	}
	if m.modelQuitState == quitConfirmationReceived {
		m.quitSuperfile(cdOnQuit)
		return tea.Quit
	}
	return cmd
}

// Update the file panel state. Change name of renamed files, filter out files
// in search, update typingb bar, etc
func (m *model) updateComponentState(msg tea.Msg) tea.Cmd {
	focusPanel := m.getFocusedFilePanel()
	var cmd tea.Cmd
	var action common.ModelAction
	switch {
	case m.firstTextInput:
		m.firstTextInput = false
	case m.fileModel.Renaming:
		focusPanel.Rename, cmd = focusPanel.Rename.Update(msg)
	case focusPanel.SearchBar.Focused():
		focusPanel.SearchBar, cmd = focusPanel.SearchBar.Update(msg)
	case m.typingModal.open:
		m.typingModal.textInput, cmd = m.typingModal.textInput.Update(msg)
	case m.promptModal.IsOpen():
		// TODO : Separate this to a utility
		cwdLocation := m.getFocusedFilePanel().Location
		action, cmd = m.promptModal.HandleUpdate(msg, cwdLocation)
		cmd = tea.Batch(cmd, m.applyPromptModalAction(action))
	case m.zoxideModal.IsOpen():
		action, cmd = m.zoxideModal.HandleUpdate(msg)
		cmd = tea.Batch(cmd, m.applyZoxideModalAction(action))
	}
	return cmd
}

// Apply the Action and notify the promptModal
func (m *model) applyPromptModalAction(action common.ModelAction) tea.Cmd {
	successMsg, cmd, actionErr := m.logAndExecuteAction(action)
	if actionErr != nil {
		m.promptModal.HandleSPFActionResults(false, actionErr.Error())
	} else if successMsg != "" {
		m.promptModal.HandleSPFActionResults(true, successMsg)
		// A successful action that opens another surface hands focus to it even
		// when the user keeps ordinary command results open. Otherwise the
		// prompt hides the preview and consumes its navigation keys.
		if promptOpensResults(action) {
			m.promptModal.Close()
		}
	}
	return cmd
}

// Utility function to log and execute actions, reducing duplication
func (m *model) logAndExecuteAction(action common.ModelAction) (string, tea.Cmd, error) {
	// Only log actions that aren't NoAction to reduce debug noise
	if _, ok := action.(common.NoAction); !ok {
		slog.Debug("Applying model action", "action", action)
	}

	switch action := action.(type) {
	case common.NoAction:
		return "", nil, nil
	case common.ShellCommandAction:
		// Shell commands are handled separately and don't return here
		m.applyShellCommandAction(action.Command)
		return "", nil, nil
	case common.SplitPanelAction:
		cmd, err := m.splitPanel()
		return "Panel successfully split", cmd, err
	case common.CDCurrentPanelAction:
		return "Panel directory changed", nil, m.updateCurrentFilePanelDir(action.Location)
	case common.OpenPanelAction:
		cmd, err := m.createNewFilePanelRelativeToCurrent(action.Location)
		return "New panel opened", cmd, err
	case common.WorkspaceAction:
		return m.executeWorkspaceAction(action)
	case common.SearchAction:
		return m.executeSavedSearchAction(action)
	case common.BatchRenameAction:
		return m.executeBatchRenameAction(action)
	case common.CatalogAction:
		return m.executeCatalogAction(action)
	case common.MetadataAction:
		return m.executeMetadataAction(action)
	default:
		return "", nil, errors.New("unhandled action type")
	}
}

func (m *model) executeWorkspaceAction(action common.WorkspaceAction) (string, tea.Cmd, error) {
	switch action.Operation {
	case common.WorkspaceSave:
		return "Workspace saved: " + action.Name, nil, m.saveWorkspace(action.Name)
	case common.WorkspaceLoad:
		err := m.loadWorkspace(action.Name)
		if err != nil {
			return "", nil, err
		}
		return "Workspace loaded: " + action.Name, m.updateComponentDimensions(), nil
	case common.WorkspaceDelete:
		return "Workspace deleted: " + action.Name, nil, m.deleteWorkspace(action.Name)
	case common.WorkspaceList:
		if m.sessionStore == nil {
			return "", nil, errors.New("workspace storage is unavailable")
		}
		names, err := m.sessionStore.ListWorkspaces()
		if err != nil {
			return "", nil, err
		}
		if len(names) == 0 {
			return "No saved workspaces", nil, nil
		}
		return "Workspaces: " + strings.Join(names, ", "), nil, nil
	default:
		return "", nil, errors.New("unknown workspace operation: " + string(action.Operation))
	}
}

// Apply the Action for zoxide modal (no result notifications needed)
func (m *model) applyZoxideModalAction(action common.ModelAction) tea.Cmd {
	_, cmd, _ := m.logAndExecuteAction(action)
	return cmd
}

// TODO : Move them around to appropriate places
func (m *model) applyShellCommandAction(shellCommand string) {
	focusPanelDir := m.getFocusedFilePanel().Location

	retCode, output, err := utils.ExecuteCommandInShell(common.DefaultCommandTimeout, focusPanelDir, shellCommand)
	m.refreshAfterDiskChange()

	m.promptModal.HandleShellCommandResults(retCode, output)

	if err != nil {
		slog.Error("Command execution failed", "retCode", retCode,
			"error", err, "output", output)
		return
	}
}

// Create a new file panel and move focus onto it. All panel-creation goes
// through here, otherwise the sidebar (or processbar/metadata) can stay focused
// next to the new panel and both react to keys. See #1497.
func (m *model) createNewFilePanel(location string) (tea.Cmd, error) {
	cmd, err := m.fileModel.CreateNewFilePanel(location)
	if err != nil {
		return cmd, err
	}
	m.focusPanel = nonePanelFocus
	return cmd, nil
}

func (m *model) splitPanel() (tea.Cmd, error) {
	return m.createNewFilePanel(m.getFocusedFilePanel().Location)
}

func (m *model) createNewFilePanelRelativeToCurrent(path string) (tea.Cmd, error) {
	currentDir := m.getFocusedFilePanel().Location
	return m.createNewFilePanel(utils.ResolveAbsPath(currentDir, path))
}

// simulates a 'cd' action
func (m *model) updateCurrentFilePanelDir(path string) error {
	panel := m.getFocusedFilePanel()
	err := panel.UpdateCurrentFilePanelDir(path)
	if err == nil {
		m.recordDirectoryVisit(panel.Location)
	}
	return err
}

func (m *model) recordDirectoryVisit(path string) {
	m.trackDirectoryWithZoxide(path)
	if err := m.sidebarModel.AddRecent(path); err != nil {
		slog.Warn("Could not save recent directory", "path", path, "error", err)
	}
}

// trackDirectoryWithZoxide adds the directory to zoxide database if zoxide is available and enabled
func (m *model) trackDirectoryWithZoxide(path string) {
	if !common.Config.ZoxideSupport || m.zClient == nil {
		return
	}

	err := m.zClient.Add(path)
	if err != nil {
		slog.Debug("Failed to add directory to zoxide", "path", path, "error", err)
	}
}

// Check if there's any processes running in background

// Triggers a warn for confirm quiting
func (m *model) warnModalForQuit() {
	m.notifyModel = notify.New(true, "Confirm to quit Xelif",
		"You still have files being processed. Are you sure you want to exit?",
		notify.QuitAction)
}

// Implement View function for bubble tea model to handle visualization.
func (m *model) View() tea.View {
	slog.Debug("model.View() called", "mainPanelHeight", m.mainPanelHeight,
		"footerHeight", m.footerHeight, "fullHeight", m.fullHeight,
		"fullWidth", m.fullWidth, "panelCount", m.fileModel.PanelCount(),
		"singlePanelWidth", m.fileModel.SinglePanelWidth,
		"maxPanels", m.fileModel.MaxFilePanel,
		"sideBarWidth", common.Config.SidebarWidth,
		"firstFilePanelWidth", m.fileModel.FilePanels[0].GetWidth())
	content := m.viewContent()
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "Xelif"
	return v
}

func (m *model) viewContent() string {
	if !m.firstLoadingComplete {
		return "Loading..."
	}

	// check is the terminal size enough
	if m.fullHeight < common.MinimumHeight || m.fullWidth < common.MinimumWidth {
		return m.terminalSizeWarnRender()
	}
	if m.fileModel.SinglePanelWidth < filepanel.MinWidth {
		return m.terminalSizeWarnAfterFirstRender()
	}

	// Do validations after min size check above. Validations will fail if user give
	// too less size to the terminal program
	if err := m.validateLayout(); err != nil {
		slog.Error("Invalid layout", "error", err)
	}

	return m.updateRenderForOverlay(m.mainComponentsRender())
}

func (m *model) updateRenderForOverlay(finalRender string) string {
	// check if need pop up modal
	if m.spfError.IsOpen() {
		return m.centerOverlay(finalRender, m.spfError.Render(), common.ModalWidth, common.ModalHeight)
	}
	if m.operations.open {
		width, height := m.operationSize()
		return m.centerOverlay(finalRender, m.renderOperations(), width, height)
	}

	if m.helpMenu.IsOpen() {
		return m.centerOverlay(finalRender, m.helpMenu.Render(), m.helpMenu.GetWidth(), m.helpMenu.GetHeight())
	}

	if m.promptModal.IsOpen() {
		return m.centerOverlay(
			finalRender,
			m.promptModalRender(),
			m.promptModal.GetWidth(),
			m.promptModal.GetMaxHeight(),
		)
	}

	if m.zoxideModal.IsOpen() {
		return m.centerOverlay(
			finalRender,
			m.zoxideModalRender(),
			m.zoxideModal.GetWidth(),
			m.zoxideModal.GetMaxHeight(),
		)
	}

	if m.globalSearchModal.IsOpen() {
		return m.centerOverlay(finalRender, m.globalSearchModal.Render(),
			m.globalSearchModal.GetWidth(), m.globalSearchModal.GetMaxHeight())
	}

	if m.commandPalette.IsOpen() {
		return m.centerOverlay(finalRender, m.commandPalette.Render(),
			m.commandPalette.GetWidth(), m.commandPalette.GetMaxHeight())
	}

	if m.textViewer.IsOpen() {
		return m.centerOverlay(finalRender, m.textViewer.Render(), m.textViewer.GetWidth(), m.textViewer.GetHeight())
	}

	if m.archiveViewer.IsOpen() {
		return m.centerOverlay(
			finalRender,
			m.archiveViewer.Render(),
			m.archiveViewer.GetWidth(),
			m.archiveViewer.GetHeight(),
		)
	}

	if m.remoteViewer.IsOpen() {
		return m.centerOverlay(
			finalRender,
			m.remoteViewer.Render(),
			m.remoteViewer.GetWidth(),
			m.remoteViewer.GetHeight(),
		)
	}

	if m.sortModal.IsOpen() {
		return m.centerOverlay(finalRender, m.sortModal.Render(), m.sortModal.Width, m.sortModal.Height)
	}

	if m.firstUse {
		welcome := m.introduceModalRender()
		return m.centerOverlay(finalRender, welcome, lipgloss.Width(welcome), lipgloss.Height(welcome))
	}

	if m.typingModal.open {
		return m.centerOverlay(finalRender, m.typineModalRender(), common.ModalWidth, common.ModalHeight)
	}

	if m.notifyModel.IsOpen() {
		return m.centerOverlay(finalRender, m.notifyModel.Render(), common.ModalWidth, common.ModalHeight)
	}

	if m.contextMenu.open {
		return stringfunction.PlaceOverlay(m.contextMenu.x, m.contextMenu.y, m.contextMenuRender(), finalRender)
	}

	return finalRender
}

func (m *model) centerOverlay(background, content string, width, height int) string {
	// Divide separately to preserve the original placement for odd dimensions.
	x := m.fullWidth/common.CenterDivisor - width/common.CenterDivisor
	y := m.fullHeight/common.CenterDivisor - height/common.CenterDivisor
	return stringfunction.PlaceOverlay(x, y, content, background)
}

func (m *model) mainComponentsRender() string {
	sidebar := m.sidebarRender()
	fileModel := m.fileModel.Render()
	mainPanel := lipgloss.JoinHorizontal(0, sidebar, fileModel)

	if !m.toggleFooter {
		return mainPanel
	}

	processBar := m.processBarRender()
	metaData := m.fileMetaData.Render(m.focusPanel == metadataFocus)
	clipboardBar := m.clipboard.Render()
	footer := lipgloss.JoinHorizontal(0, processBar, metaData, clipboardBar)
	return lipgloss.JoinVertical(0, mainPanel, footer)
}

// Close superfile application. Cd into the current dir if CdOnQuit on and save
// the path in state direcotory
func (m *model) quitSuperfile(cdOnQuit bool) {
	m.processBarModel.Shutdown()
	m.processBarModel.WaitWorkers()
	if common.Config.RestoreLastSession {
		if err := m.saveLastSession(); err != nil {
			slog.Error("Could not save the current session", "error", err)
		}
	}

	// Resource cleanup
	if common.Config.Metadata && et != nil {
		_ = et.Close()
	}
	if m.folderWatcher != nil {
		if err := m.folderWatcher.Close(); err != nil {
			slog.Error("Could not stop folder watcher", "error", err)
		}
	}
	if err := m.archiveViewer.Close(); err != nil {
		slog.Error("Could not close archive viewer", "error", err)
	}
	if m.duplicateScanCancel != nil {
		m.duplicateScanCancel()
		m.duplicateScanCancel = nil
	}
	if m.dirCompareCancel != nil {
		m.dirCompareCancel()
		m.dirCompareCancel = nil
	}
	if m.remoteCancel != nil {
		m.remoteCancel()
		m.remoteCancel = nil
	}
	m.cancelPluginCommand()
	m.fileModel.FilePreview.CleanUp()

	// cd on quit
	currentDir := m.getFocusedFilePanel().Location
	variable.SetLastDir(currentDir)

	if cdOnQuit {
		// escape single quote
		currentDir = strings.ReplaceAll(currentDir, "'", "'\\''")
		err := os.WriteFile(variable.LastDirFile, []byte("cd '"+currentDir+"'"), utils.ConfigFilePerm)
		if err != nil {
			slog.Error("Error during writing lastdir file", "error", err)
		}
	}
	m.modelQuitState = quitDone
	slog.Debug("Quitting superfile", "current dir", currentDir)
}

// thread safe. returns current ioReqCnt and increments it.
func (m *model) nextIoReqCnt() int {
	var res = atomic.AddInt32(&m.ioReqCnt, 1)
	return int(res)
}

func (m *model) updateModelStateForMessage(msg tea.Msg) {
	if _, polling := msg.(refreshTickMsg); polling {
		_ = m.processBarModel.ObserveSync(
			processbar.TaskSpec{Kind: "Directory refresh check", Background: true, GroupKey: "directory-poll"},
			func() error {
				m.updateModelStateAfterMsg()
				return nil
			},
		)
	} else {
		m.updateModelStateAfterMsg()
	}
}
