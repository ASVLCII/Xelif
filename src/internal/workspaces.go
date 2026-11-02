package internal

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/session"
	"github.com/yorukot/superfile/src/internal/ui/commandpalette"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
	"github.com/yorukot/superfile/src/internal/ui/globalsearch"
	"github.com/yorukot/superfile/src/internal/ui/metadata"
	"github.com/yorukot/superfile/src/internal/ui/workspacetabs"
)

type workspaceRuntime struct {
	State  session.WorkspaceState
	Views  []filepanel.ViewState
	Focus  focusPanelType
	Search globalsearch.SearchSnapshot
}

const (
	workspaceSwitchPrefix = "workspace_switch:"
	maxClosedWorkspaces   = 10

	workspaceActionRename = "workspace_rename"
	workspaceActionClose  = "workspace_close"
	workspaceActionLeft   = "workspace_left"
	workspaceActionRight  = "workspace_right"
	workspaceActionNew    = "workspace_new"
	workspaceActionReopen = "workspace_reopen"

	workspaceGoneNotice               = "Workspace is no longer open"
	workspaceCompactWidth             = 80
	workspacePickerExtraItems         = 8
	workspaceRenameWidth              = 36
	contextMenuSidebarGap             = 2
	contextMenuEntryRow               = 5
	workspaceOperationsContinueNotice = "Workspace closed; file operations continue in Operations"
)

func (m *model) workspaceIndexByID(id string) int {
	if id == "" {
		return -1
	}
	for i, w := range m.workspaces {
		if w.State.ID == id {
			return i
		}
	}
	return -1
}

// cloneWorkspaceRuntime keeps closed-workspace history independent of the live
// workspace and of any later panel refresh.
func cloneWorkspaceRuntime(w workspaceRuntime) workspaceRuntime {
	clone := w
	clone.State.Layout = session.CloneState(w.State.Layout)
	clone.Views = make([]filepanel.ViewState, len(w.Views))
	for i, view := range w.Views {
		clone.Views[i] = view
		clone.Views[i].Selected = maps.Clone(view.Selected)
		clone.Views[i].SelectionOrder = maps.Clone(view.SelectionOrder)
		clone.Views[i].Records = maps.Clone(view.Records)
	}
	clone.Search.Results = slices.Clone(w.Search.Results)
	clone.Search.SavedResults = slices.Clone(w.Search.SavedResults)
	return clone
}

func (m *model) rememberClosedWorkspace(w workspaceRuntime) {
	m.closedWorkspaces = append(m.closedWorkspaces, cloneWorkspaceRuntime(w))
	if len(m.closedWorkspaces) > maxClosedWorkspaces {
		m.closedWorkspaces = append(
			[]workspaceRuntime(nil),
			m.closedWorkspaces[len(m.closedWorkspaces)-maxClosedWorkspaces:]...,
		)
	}
}

func (m *model) freshLaunchWorkspace() tea.Cmd {
	id, err := session.NewWorkspaceID()
	if err != nil {
		m.showUserError("Could not create workspace", err)
		return nil
	}
	m.workspaces = []workspaceRuntime{{
		State: session.WorkspaceState{
			ID:    id,
			Title: m.workspaceTitle(m.launchDirectory),
			Layout: session.State{
				Version:      session.CurrentVersion,
				ShowDotFiles: m.fileModel.DisplayDotFiles,
				Panels:       []session.PanelState{{Tabs: []session.TabState{{Location: m.launchDirectory}}}},
			},
		},
	}}
	m.activeWorkspace = 0
	return m.restoreActiveWorkspace()
}

func (m *model) ensureWorkspaces() {
	if len(m.workspaces) != 0 {
		return
	}
	m.launchDirectory = m.getFocusedFilePanel().Location
	id, err := session.NewWorkspaceID()
	if err != nil {
		m.showUserError("Could not initialize workspace", err)
		return
	}
	m.workspaces = []workspaceRuntime{
		{
			State: session.WorkspaceState{
				ID:     id,
				Title:  m.workspaceTitle(m.launchDirectory),
				Layout: m.snapshotSession(),
			},
		},
	}
}

func (m *model) workspaceTitle(path string) string {
	base := filepath.Base(filepath.Clean(path))
	if base == "." || base == "" || base == string(filepath.Separator) {
		base = "Home"
	}
	return m.uniqueWorkspaceTitle(base, -1)
}

func (m *model) uniqueWorkspaceTitle(base string, except int) string {
	for suffix := 1; ; suffix++ {
		suffixText := ""
		if suffix > 1 {
			suffixText = fmt.Sprintf(" (%d)", suffix)
		}
		candidate := boundedWorkspaceTitle(base, suffixText)
		found := false
		for i, w := range m.workspaces {
			if i != except && w.State.Title == candidate {
				found = true
				break
			}
		}
		if !found {
			return candidate
		}
	}
}

func (m *model) captureWorkspace() {
	if m.activeWorkspace < 0 || m.activeWorkspace >= len(m.workspaces) {
		return
	}
	w := &m.workspaces[m.activeWorkspace]
	w.State.Layout = m.snapshotSession()
	w.Focus = m.focusPanel
	w.Search = m.globalSearchModal.Snapshot()
	w.Views = make([]filepanel.ViewState, len(m.fileModel.FilePanels))
	for i := range m.fileModel.FilePanels {
		w.Views[i] = m.fileModel.FilePanels[i].SnapshotView()
	}
}

func (m *model) workspaceSwitchAllowed() bool {
	if m.fileModel.Renaming || m.sidebarModel.IsRenaming() || m.typingModal.open || m.promptModal.IsOpen() ||
		m.notifyModel.IsOpen() || m.spfError.IsOpen() || m.workspaceRenaming || m.firstUse {
		m.workspaceNotice = "Finish or cancel the current dialog before switching workspaces"
		return false
	}
	return true
}

func (m *model) cancelWorkspaceWork() {
	m.workspaceGeneration++
	m.cancelWorkspaceToolViews()
	m.fileModel.CancelEphemeral()
	m.globalSearchModal.Close()
	if m.metadataCancel != nil {
		m.metadataCancel()
		m.metadataCancel = nil
	}
	m.metadataReqID = -1
	m.pendingMetadataPatch = nil
	m.gitStatusReqID++
	if m.gitStatusCancel != nil {
		m.gitStatusCancel()
		m.gitStatusCancel = nil
	}
	m.gitStatusPaths = nil
	m.fileMetaData = metadata.New()
	m.commandPalette.Close()
	m.contextMenu.open = false
	m.sortModal.Close()
}

type workspaceMessage struct {
	ID         string
	Generation uint64
	Message    tea.Msg
}

// Scope only ephemeral work. File mutations and their registry completions are
// intentionally never wrapped, so closing a workspace cannot cancel an operation.
func (m *model) workspaceCommand(cmd tea.Cmd) tea.Cmd {
	if cmd == nil || len(m.workspaces) == 0 {
		return cmd
	}
	return scopeWorkspaceCommand(cmd, m.workspaces[m.activeWorkspace].State.ID, m.workspaceGeneration)
}

func scopeWorkspaceCommand(cmd tea.Cmd, id string, generation uint64) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			commands := make(tea.BatchMsg, len(batch))
			for i, c := range batch {
				commands[i] = scopeWorkspaceCommand(c, id, generation)
			}
			return commands
		}
		if msg == nil {
			return nil
		}
		return workspaceMessage{ID: id, Generation: generation, Message: msg}
	}
}

func (m *model) activateWorkspace(index int) tea.Cmd {
	if index < 0 || index >= len(m.workspaces) || index == m.activeWorkspace {
		return nil
	}
	if !m.workspaceSwitchAllowed() {
		return nil
	}
	m.captureWorkspace()
	m.cancelWorkspaceWork()
	m.activeWorkspace = index
	return m.restoreActiveWorkspace()
}

func (m *model) restoreActiveWorkspace() tea.Cmd {
	w := &m.workspaces[m.activeWorkspace]
	s := w.State.Layout
	m.fileModel.RestoreWorkspace(s.Panels, s.FocusedPanel, s.ShowDotFiles, m.launchDirectory)
	m.fileModel.FilePreview.SetOpen(s.PreviewVisible)
	m.toggleFooter = s.FooterVisible
	m.focusPanel = w.Focus
	searchCmd := m.globalSearchModal.Restore(w.Search)
	m.setHeightValues()
	cmd := m.updateComponentDimensions()
	for i := range m.fileModel.FilePanels {
		if i < len(w.Views) {
			m.fileModel.FilePanels[i].RestoreView(w.Views[i], s.ShowDotFiles)
		} else {
			m.fileModel.FilePanels[i].UpdateElementsIfNeeded(true, s.ShowDotFiles)
		}
	}
	return tea.Batch(cmd, searchCmd, m.syncFolderWatcherCmd(), m.refreshGitStatusCmd(true))
}

func (m *model) newWorkspace(path string) tea.Cmd {
	m.ensureWorkspaces()
	if !m.canAddWorkspace() {
		return nil
	}
	if !m.workspaceSwitchAllowed() {
		return nil
	}
	id, err := session.NewWorkspaceID()
	if err != nil {
		m.showUserError("Could not create workspace", err)
		return nil
	}
	m.captureWorkspace()
	m.cancelWorkspaceWork()
	layout := session.State{Version: session.CurrentVersion, ShowDotFiles: m.fileModel.DisplayDotFiles,
		Panels: []session.PanelState{{Tabs: []session.TabState{{Location: path}}, SortKind: 0}}}
	m.workspaces = append(
		m.workspaces,
		workspaceRuntime{State: session.WorkspaceState{ID: id, Title: m.workspaceTitle(path), Layout: layout}},
	)
	m.activeWorkspace = len(m.workspaces) - 1
	return m.restoreActiveWorkspace()
}

func (m *model) closeWorkspace() tea.Cmd {
	if m.activeWorkspace < 0 || m.activeWorkspace >= len(m.workspaces) {
		return nil
	}
	return m.closeWorkspaceByID(m.workspaces[m.activeWorkspace].State.ID)
}

// closeWorkspaceByID closes one stable workspace identity. Closing an inactive
// target leaves the active workspace, its inputs, and its ephemeral work alone.
func (m *model) closeWorkspaceByID(id string) tea.Cmd {
	if m.workspaceRenaming || m.firstUse || m.appearanceOpen {
		return nil
	}
	m.ensureWorkspaces()
	index := m.workspaceIndexByID(id)
	if index < 0 {
		m.workspaceNotice = workspaceGoneNotice
		return nil
	}
	active := index == m.activeWorkspace
	if active {
		if !m.workspaceSwitchAllowed() {
			return nil
		}
		m.captureWorkspace()
		m.cancelWorkspaceWork()
	}
	m.rememberClosedWorkspace(m.workspaces[index])
	m.workspaces = append(m.workspaces[:index], m.workspaces[index+1:]...)
	if len(m.workspaces) == 0 {
		cmd := m.freshLaunchWorkspace()
		if m.processBarModel.HasRunningProcesses() {
			m.workspaceNotice = workspaceOperationsContinueNotice
		}
		return cmd
	}
	if active {
		// Stay near the closed position: prefer the left neighbor, then the right.
		m.activeWorkspace = min(max(index-1, 0), len(m.workspaces)-1)
		if m.processBarModel.HasRunningProcesses() {
			m.workspaceNotice = workspaceOperationsContinueNotice
		}
		return m.restoreActiveWorkspace()
	}
	if index < m.activeWorkspace {
		m.activeWorkspace--
	}
	if m.processBarModel.HasRunningProcesses() {
		m.workspaceNotice = workspaceOperationsContinueNotice
	}
	return nil
}

func (m *model) reopenWorkspace() tea.Cmd {
	if !m.canAddWorkspace() {
		return nil
	}
	if len(m.closedWorkspaces) == 0 || !m.workspaceSwitchAllowed() {
		return nil
	}
	m.captureWorkspace()
	m.cancelWorkspaceWork()
	w := m.closedWorkspaces[len(m.closedWorkspaces)-1]
	m.closedWorkspaces = m.closedWorkspaces[:len(m.closedWorkspaces)-1]
	w.State.Title = m.uniqueWorkspaceTitle(w.State.Title, -1)
	m.workspaces = append(m.workspaces, w)
	m.activeWorkspace = len(m.workspaces) - 1
	return m.restoreActiveWorkspace()
}

func (m *model) moveWorkspace(delta int) {
	if m.activeWorkspace < 0 || m.activeWorkspace >= len(m.workspaces) {
		return
	}
	m.moveWorkspaceByID(m.workspaces[m.activeWorkspace].State.ID, delta)
}

// moveWorkspaceByID reorders by stable identity and keeps the active workspace
// active, whether or not the moved target is the active one.
func (m *model) moveWorkspaceByID(id string, delta int) {
	index := m.workspaceIndexByID(id)
	if index < 0 {
		m.workspaceNotice = workspaceGoneNotice
		return
	}
	next := index + delta
	if next < 0 || next >= len(m.workspaces) {
		return
	}
	m.workspaces[next], m.workspaces[index] = m.workspaces[index], m.workspaces[next]
	switch m.activeWorkspace {
	case index:
		m.activeWorkspace = next
	case next:
		m.activeWorkspace = index
	}
}

func (m *model) workspaceLayout() workspacetabs.Layout {
	tabs := make([]workspacetabs.Tab, len(m.workspaces))
	for i, w := range m.workspaces {
		tabs[i] = workspacetabs.Tab{ID: w.State.ID, Title: w.State.Title}
	}
	return workspacetabs.Compute(tabs, m.activeWorkspace, m.fullWidth)
}

func (m *model) renderWorkspaceStrip() string {
	return m.workspaceLayout().Render(func(s string, active bool) string {
		style := common.FilePanelTopPathStyle
		if active {
			style = style.Reverse(true).Bold(true)
		} else {
			style = style.Faint(true)
		}
		return style.Render(s)
	})
}

func (m *model) compactStatus() string {
	panel := m.getFocusedFilePanel()
	text := fmt.Sprintf(" %d items", panel.ElemCount())
	if panel.SelectedCount() > 0 {
		text += fmt.Sprintf("  |  %d selected", panel.SelectedCount())
	}
	if m.workspaceNotice != "" {
		text = " " + m.workspaceNotice
	}
	if m.fullWidth < workspaceCompactWidth {
		text = " Sidebar |" + text
	}
	return lipgloss.NewStyle().Width(m.fullWidth).Render(ansi.Truncate(text, m.fullWidth, "..."))
}

func (m *model) openWorkspacePicker() tea.Cmd {
	items := make([]commandpalette.Item, 0, len(m.workspaces)+workspacePickerExtraItems)
	for i, w := range m.workspaces {
		label := w.State.Title
		if i == m.activeWorkspace {
			label += " (active)"
		}
		items = append(items, paletteItem(workspaceSwitchPrefix+w.State.ID, label, "Workspaces", nil))
	}
	items = append(items, m.workspaceCommandItems()...)
	m.commandPalette = commandpalette.New(items)
	m.setCommandPaletteSize()
	return m.commandPalette.Open()
}

func (m *model) workspaceCommandItems() []commandpalette.Item {
	return []commandpalette.Item{
		paletteItem(workspaceActionNew, "New workspace", "Workspaces", nil),
		paletteItem(workspaceActionClose, "Close workspace", "Workspaces", nil),
		paletteItem(workspaceActionRename, "Rename workspace", "Workspaces", nil),
		paletteItem(workspaceActionLeft, "Move workspace left", "Workspaces", nil),
		paletteItem(workspaceActionRight, "Move workspace right", "Workspaces", nil),
		paletteItem(workspaceActionReopen, "Reopen closed workspace", "Workspaces", nil),
		paletteItem("workspace_picker", "Workspaces picker", "Workspaces", nil),
		paletteItem("panel_folders", "Folders in this panel", "Navigation", nil),
		paletteItem("search_resume", "Return to search results", "Search", nil),
		paletteItem("appearance_setup", "Appearance and setup", "Settings", nil),
		paletteItem("sidebar_drawer", "Sidebar drawer", "Navigation", nil, "favourites", "disks", "home"),
	}
}

func (m *model) applyWorkspaceCommand(id string) (tea.Cmd, bool) {
	if strings.HasPrefix(id, "sidebar_path:") {
		if err := m.updateCurrentFilePanelDir(strings.TrimPrefix(id, "sidebar_path:")); err != nil {
			m.showUserError("Could not open folder", err)
		}
		return nil, true
	}
	if strings.HasPrefix(id, workspaceSwitchPrefix) {
		for i, w := range m.workspaces {
			if w.State.ID == strings.TrimPrefix(id, workspaceSwitchPrefix) {
				return m.activateWorkspace(i), true
			}
		}
		return nil, true
	}
	if strings.HasPrefix(id, "panel_folder:") {
		i, err := strconv.Atoi(strings.TrimPrefix(id, "panel_folder:"))
		if err == nil {
			err = m.getFocusedFilePanel().ActivateTab(i)
		}
		if err != nil {
			m.showUserError("Could not open folder", err)
		}
		return nil, true
	}
	switch id {
	case "sidebar_drawer":
		return m.openSidebarDrawer(), true
	case "appearance_setup":
		m.openAppearance()
		return nil, true
	case "search_resume":
		return m.globalSearchModal.Resume(), true
	case workspaceActionNew:
		return m.newWorkspace(m.getFocusedFilePanel().Location), true
	case workspaceActionClose:
		return m.closeWorkspace(), true
	case workspaceActionReopen:
		return m.reopenWorkspace(), true
	case workspaceActionLeft:
		m.moveWorkspace(-1)
		return nil, true
	case workspaceActionRight:
		m.moveWorkspace(1)
		return nil, true
	case "workspace_picker":
		return m.openWorkspacePicker(), true
	case workspaceActionRename:
		return m.beginActiveWorkspaceRename(), true
	case "panel_folders":
		state := m.getFocusedFilePanel().SnapshotSession()
		items := make([]commandpalette.Item, 0, len(state.Tabs))
		for i, t := range state.Tabs {
			items = append(
				items,
				paletteItem("panel_folder:"+strconv.Itoa(i), t.Location, "Folders in this panel", nil),
			)
		}
		m.commandPalette = commandpalette.New(items)
		m.setCommandPaletteSize()
		return m.commandPalette.Open(), true
	}
	return nil, false
}

// applyWorkspaceContextCommand executes a strip-menu action by stable target ID.
// New and reopen are global; the rest never activate an inactive target.
func (m *model) applyWorkspaceContextCommand(workspaceID, id string) tea.Cmd {
	switch id {
	case workspaceActionRename:
		return m.beginWorkspaceRename(workspaceID)
	case workspaceActionClose:
		return m.closeWorkspaceByID(workspaceID)
	case workspaceActionLeft:
		m.moveWorkspaceByID(workspaceID, -1)
	case workspaceActionRight:
		m.moveWorkspaceByID(workspaceID, 1)
	case workspaceActionNew:
		return m.newWorkspace(m.getFocusedFilePanel().Location)
	case workspaceActionReopen:
		return m.reopenWorkspace()
	}
	return nil
}

func (m *model) beginActiveWorkspaceRename() tea.Cmd {
	if m.activeWorkspace < 0 || m.activeWorkspace >= len(m.workspaces) {
		return nil
	}
	return m.beginWorkspaceRename(m.workspaces[m.activeWorkspace].State.ID)
}

func (m *model) beginWorkspaceRename(id string) tea.Cmd {
	if !m.workspaceSwitchAllowed() {
		return nil
	}
	index := m.workspaceIndexByID(id)
	if index < 0 {
		m.workspaceNotice = workspaceGoneNotice
		return nil
	}
	m.workspaceRenameID = id
	m.workspaceRenameError = ""
	m.workspaceNameInput = textinput.New()
	m.workspaceNameInput.SetValue(m.workspaces[index].State.Title)
	m.workspaceNameInput.CharLimit = 128
	m.workspaceNameInput.SetWidth(workspaceRenameWidth)
	m.workspaceRenaming = true
	return m.workspaceNameInput.Focus()
}

func (m *model) openSidebarDrawer() tea.Cmd {
	items := make([]commandpalette.Item, 0)
	for _, location := range m.sidebarModel.Locations() {
		items = append(
			items,
			paletteItem("sidebar_path:"+location.Path, location.Name, location.Section, nil, location.Path),
		)
	}
	m.commandPalette = commandpalette.New(items)
	m.setCommandPaletteSize()
	return m.commandPalette.Open()
}

func (m *model) workspaceRenameUpdate(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case browserEscapeKey:
			m.workspaceRenaming = false
			m.workspaceRenameID = ""
			return nil
		case browserEnterKey:
			m.commitWorkspaceRename()
			return nil
		}
	}
	var cmd tea.Cmd
	m.workspaceNameInput, cmd = m.workspaceNameInput.Update(msg)
	return cmd
}

func (m *model) commitWorkspaceRename() {
	id := m.workspaceRenameID
	if id == "" && m.activeWorkspace >= 0 && m.activeWorkspace < len(m.workspaces) {
		id = m.workspaces[m.activeWorkspace].State.ID
	}
	index := m.workspaceIndexByID(id)
	if index < 0 {
		m.workspaceRenaming = false
		m.workspaceRenameID = ""
		m.showUserError(workspaceGoneNotice, errors.New("the workspace was closed while renaming"))
		return
	}
	title := strings.TrimSpace(ansi.Strip(m.workspaceNameInput.Value()))
	if issue := workspaceRenameValidation(title); issue != "" {
		m.workspaceRenameError = issue
		return
	}
	m.workspaces[index].State.Title = m.uniqueWorkspaceTitle(title, index)
	m.workspaceRenaming = false
	m.workspaceRenameID = ""
}

// Existing bindings take precedence over newly introduced workspace shortcuts.
// This matters when an older/custom profile still uses Ctrl+T for panel folders.
func workspaceShortcutAvailable(key string) bool {
	return !workspaceShortcutConflict(key, reflect.ValueOf(common.Hotkeys))
}

func workspaceShortcutConflict(key string, hotkeys reflect.Value) bool {
	for i := range hotkeys.NumField() {
		switch hotkeys.Type().Field(i).Name {
		case "NewWorkspace",
			"CloseWorkspace",
			"NextWorkspace",
			"PreviousWorkspace",
			"CycleFocus",
			"ReverseCycleFocus",
			"ContextMenu":
			continue
		}
		if keys, ok := hotkeys.Field(i).Interface().([]string); ok && containsKey(keys, key) {
			return true
		}
	}
	return false
}

func (m *model) handleWorkspaceKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if len(m.workspaces) == 0 || m.IsOverlayModelOpen() || m.spfError.IsOpen() || m.fileModel.Renaming ||
		m.sidebarModel.IsRenaming() || m.getFocusedFilePanel().SearchBar.Focused() || m.sidebarModel.SearchBarFocused() {
		return nil, false
	}
	key := msg.String()
	if !workspaceShortcutAvailable(key) {
		return nil, false
	}
	switch {
	case containsKey(common.Hotkeys.NewWorkspace, key):
		return m.newWorkspace(m.getFocusedFilePanel().Location), true
	case containsKey(common.Hotkeys.CloseWorkspace, key):
		return m.closeWorkspace(), true
	case containsKey(common.Hotkeys.NextWorkspace, key):
		return m.activateWorkspace((m.activeWorkspace + 1) % len(m.workspaces)), true
	case containsKey(common.Hotkeys.PreviousWorkspace, key):
		return m.activateWorkspace((m.activeWorkspace + len(m.workspaces) - 1) % len(m.workspaces)), true
	case containsKey(common.Hotkeys.CycleFocus, key):
		m.cycleComponentFocus(1)
		return nil, true
	case containsKey(common.Hotkeys.ReverseCycleFocus, key):
		m.cycleComponentFocus(-1)
		return nil, true
	case containsKey(common.Hotkeys.ContextMenu, key):
		p := m.getFocusedFilePanel()
		if !p.Empty() {
			m.openContextMenu(
				tea.Mouse{X: m.sidebarModel.GetWidth() + contextMenuSidebarGap, Y: contextMenuEntryRow},
				m.fileModel.FocusedPanelIndex,
				p.GetFocusedItem().Location,
			)
		}
		return nil, true
	}
	return nil, false
}

func (m *model) cycleComponentFocus(delta int) {
	foci := []focusPanelType{nonePanelFocus}
	if !m.sidebarModel.Disabled() {
		foci = append(foci, sidebarFocus)
	}
	if m.toggleFooter {
		foci = append(foci, processBarFocus, metadataFocus)
	}
	index := 0
	for i, f := range foci {
		if f == m.focusPanel {
			index = i
		}
	}
	m.focusPanel = foci[(index+delta+len(foci))%len(foci)]
	m.getFocusedFilePanel().IsFocused = m.focusPanel == nonePanelFocus
	m.sidebarModel.SearchBarBlur()
}
