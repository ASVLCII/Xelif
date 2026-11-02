package internal

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
)

func directionalKey(code rune, modifier tea.KeyMod) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: modifier}
}

func TestDirectionalWorkspaceDefaults(t *testing.T) {
	require.Contains(t, common.Hotkeys.NextWorkspace, "ctrl+right")
	require.Contains(t, common.Hotkeys.PreviousWorkspace, "ctrl+left")
	require.NotContains(t, common.Hotkeys.NextFileTab, "ctrl+right")
	require.NotContains(t, common.Hotkeys.PreviousFileTab, "ctrl+left")
	require.Contains(t, common.Hotkeys.NextFilePanel, "shift+right")
	require.Contains(t, common.Hotkeys.PreviousFilePanel, "shift+left")
	require.Contains(t, common.Hotkeys.NextFileTab, directionalKey(tea.KeyRight, tea.ModAlt|tea.ModShift).String())
	require.Contains(t, common.Hotkeys.PreviousFileTab, directionalKey(tea.KeyLeft, tea.ModAlt|tea.ModShift).String())
}

func TestDirectionalWorkspaceArrowsCycleAndPreserveAlternatives(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	_ = m.newWorkspace(t.TempDir())
	require.Equal(t, 1, m.activeWorkspace)
	_ = m.handleKeyInput(directionalKey(tea.KeyLeft, tea.ModCtrl))
	require.Zero(t, m.activeWorkspace)
	_ = m.handleKeyInput(directionalKey(tea.KeyRight, tea.ModCtrl))
	require.Equal(t, 1, m.activeWorkspace)
	_ = m.handleKeyInput(directionalKey(tea.KeyRight, tea.ModCtrl))
	require.Zero(t, m.activeWorkspace)
	_ = m.handleKeyInput(directionalKey(tea.KeyTab, tea.ModCtrl))
	require.Equal(t, 1, m.activeWorkspace)
}

func TestDirectionalPaneArrowsFollowHorizontalOrder(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	_, err := m.fileModel.CreateNewFilePanel(t.TempDir())
	require.NoError(t, err)
	require.True(t, m.fileModel.FocusPanel(0))
	require.False(t, m.sidebarModel.Disabled())
	left := directionalKey(tea.KeyLeft, tea.ModShift)
	right := directionalKey(tea.KeyRight, tea.ModShift)
	_ = m.handleKeyInput(left)
	require.Equal(t, sidebarFocus, m.focusPanel)
	require.False(t, m.getFocusedFilePanel().IsFocused)
	_ = m.handleKeyInput(left)
	require.Equal(t, sidebarFocus, m.focusPanel, "left edge must not jump to the rightmost pane")
	_ = m.handleKeyInput(right)
	require.Equal(t, nonePanelFocus, m.focusPanel)
	require.Zero(t, m.fileModel.FocusedPanelIndex)
	require.True(t, m.getFocusedFilePanel().IsFocused)
	_ = m.handleKeyInput(right)
	require.Equal(t, 1, m.fileModel.FocusedPanelIndex)
	_ = m.handleKeyInput(right)
	require.Equal(t, 1, m.fileModel.FocusedPanelIndex, "right edge must not wrap to the sidebar")
	_ = m.handleKeyInput(left)
	require.Zero(t, m.fileModel.FocusedPanelIndex)
	_ = m.handleKeyInput(directionalKey(tea.KeyTab, 0))
	require.Equal(t, 1, m.fileModel.FocusedPanelIndex)
	_ = m.handleKeyInput(directionalKey(tea.KeyTab, 0))
	require.Zero(t, m.fileModel.FocusedPanelIndex, "Tab retains file-only cycling")
	require.Equal(t, nonePanelFocus, m.focusPanel)
}

func TestDirectionalPaneArrowOpensCollapsedSidebar(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	_ = m.handleWindowResize(tea.WindowSizeMsg{Width: 60, Height: 24})
	require.False(t, m.commandPalette.IsOpen())
	_ = m.handleKeyInput(directionalKey(tea.KeyLeft, tea.ModShift))
	require.True(t, m.commandPalette.IsOpen(), "collapsed sidebar must remain keyboard reachable")
}

func TestDirectionalArrowsDoNotStealTextEditing(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	_ = m.newWorkspace(t.TempDir())
	panel := m.getFocusedFilePanel()
	panel.SearchBar.SetValue("keep these words")
	panel.SearchBar.Focus()
	_ = m.handleKeyInput(directionalKey(tea.KeyLeft, tea.ModCtrl))
	_ = m.handleKeyInput(directionalKey(tea.KeyLeft, tea.ModShift))
	require.Equal(t, 1, m.activeWorkspace)
	require.Equal(t, nonePanelFocus, m.focusPanel)
	require.True(t, panel.SearchBar.Focused())
	require.Equal(t, "keep these words", panel.SearchBar.Value())
	panel.SearchBar.Blur()
	m.fileModel.Renaming = true
	_ = m.handleKeyInput(directionalKey(tea.KeyLeft, tea.ModCtrl))
	_ = m.handleKeyInput(directionalKey(tea.KeyLeft, tea.ModShift))
	require.Equal(t, 1, m.activeWorkspace)
	require.Equal(t, nonePanelFocus, m.focusPanel)
	require.True(t, m.fileModel.Renaming)
}

func TestDirectionalWorkspaceCustomFolderBindingKeepsPrecedence(t *testing.T) {
	previous := common.Hotkeys.PreviousFileTab
	t.Cleanup(func() { common.Hotkeys.PreviousFileTab = previous })
	common.Hotkeys.PreviousFileTab = []string{"ctrl+left"}
	m := dailyDriverTestModel(t.TempDir())
	_ = m.newWorkspace(t.TempDir())
	panel := m.getFocusedFilePanel()
	require.NoError(t, panel.NewTab(t.TempDir()))
	require.Equal(t, 1, panel.ActiveTabIndex())
	_ = m.handleKeyInput(directionalKey(tea.KeyLeft, tea.ModCtrl))
	require.Equal(t, 1, m.activeWorkspace, "explicit conflicting custom bindings retain precedence")
	require.Zero(t, panel.ActiveTabIndex(), "the custom folder-tab command must still run")
}

func TestDirectionalPaneArrowsRespectHiddenSidebar(t *testing.T) {
	previous := common.Config.SidebarWidth
	t.Cleanup(func() { common.Config.SidebarWidth = previous })
	common.Config.SidebarWidth = 0
	m := dailyDriverTestModel(t.TempDir())
	_ = m.handleKeyInput(directionalKey(tea.KeyLeft, tea.ModShift))
	require.Equal(t, nonePanelFocus, m.focusPanel)
	require.False(t, m.commandPalette.IsOpen())
	require.True(t, m.getFocusedFilePanel().IsFocused)
}

func TestDirectionalWorkspaceArrowsRespectSearchOverlay(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	_ = m.newWorkspace(t.TempDir())
	_ = m.globalSearchModal.Open(m.getFocusedFilePanel().Location)
	_ = m.handleKeyInput(directionalKey(tea.KeyLeft, tea.ModCtrl))
	_ = m.handleKeyInput(directionalKey(tea.KeyLeft, tea.ModShift))
	require.Equal(t, 1, m.activeWorkspace)
	require.Equal(t, nonePanelFocus, m.focusPanel)
	require.True(t, m.globalSearchModal.IsOpen())
}

func TestDirectionalPaneArrowsDoNotRedirectFooterFocus(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	for _, focus := range []focusPanelType{processBarFocus, metadataFocus} {
		m.focusPanel = focus
		m.getFocusedFilePanel().IsFocused = false
		_ = m.handleKeyInput(directionalKey(tea.KeyLeft, tea.ModShift))
		_ = m.handleKeyInput(directionalKey(tea.KeyRight, tea.ModShift))
		require.Equal(t, focus, m.focusPanel)
		require.False(t, m.getFocusedFilePanel().IsFocused)
	}
}
