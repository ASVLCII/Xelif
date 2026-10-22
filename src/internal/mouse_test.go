package internal

import (
	"fmt"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
	"github.com/yorukot/superfile/src/internal/ui/notify"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func fileEntryScreenPosition(m *model, elementIndex int) (int, int) {
	x := m.sidebarModel.GetWidth() + 1 // inside the first panel's left border

	panel := &m.fileModel.FilePanels[0]
	y := 4
	if panel.NeedRenderHeaders() {
		y++
	}
	y += elementIndex - panel.GetRenderIndex()
	return x, y
}

func clickAt(x, y int, button tea.MouseButton) tea.MouseClickMsg {
	return tea.MouseClickMsg{X: x, Y: y, Button: button}
}

func TestMouseWheelMovesExactlyOneFile(t *testing.T) {
	dir := t.TempDir()
	for i := range 20 {
		utils.SetupFiles(t, filepath.Join(dir, fmt.Sprintf("file-%02d.txt", i)))
	}
	m := defaultTestModel(dir)
	m.focusPanel = nonePanelFocus
	panel := m.getFocusedFilePanel()
	require.True(t, panel.FocusIndex(8))
	x, y := fileEntryScreenPosition(m, 8)
	for _, direction := range []struct {
		button tea.MouseButton
		delta  int
	}{{tea.MouseWheelDown, 1}, {tea.MouseWheelUp, -1}} {
		for range 4 {
			before := panel.GetCursor()
			TeaUpdate(m, tea.MouseWheelMsg{X: x, Y: y, Button: direction.button})
			require.Equal(t, before+direction.delta, panel.GetCursor())
		}
	}
}

func TestMouseWheelListDeltaIsOne(t *testing.T) {
	require.Equal(t, -1, mouseWheelDelta(tea.MouseWheelUp))
	require.Equal(t, 1, mouseWheelDelta(tea.MouseWheelDown))
	require.Zero(t, mouseWheelDelta(tea.MouseLeft))
}

func TestMouseWheelTargetsHoveredSidebarAndDoesNotWrapFiles(t *testing.T) {
	dir := t.TempDir()
	utils.SetupFiles(t, filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt"))
	m := defaultTestModel(dir)
	m.focusPanel = nonePanelFocus
	panel := m.getFocusedFilePanel()
	before := panel.GetCursor()
	TeaUpdate(m, tea.MouseWheelMsg{X: 2, Y: 6, Button: tea.MouseWheelDown})
	require.Equal(t, sidebarFocus, m.focusPanel)
	require.Equal(t, before, panel.GetCursor(), "sidebar wheel must not move the file list")
	x, y := fileEntryScreenPosition(m, 0)
	require.True(t, panel.FocusIndex(0))
	TeaUpdate(m, tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelUp})
	require.Equal(t, nonePanelFocus, m.focusPanel)
	require.Zero(t, panel.GetCursor(), "wheel at first file must not wrap to last")
	require.True(t, panel.FocusIndex(panel.ElemCount()-1))
	before = panel.GetCursor()
	TeaUpdate(m, tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelDown})
	require.Equal(t, before, panel.GetCursor())
}

func TestMouseClickFocusesVisibleFileRow(t *testing.T) {
	dir := t.TempDir()
	utils.SetupFiles(t, filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt"))
	m := defaultTestModel(dir)
	panel := m.getFocusedFilePanel()
	target := panel.FindElementIndexByName("b.txt")
	require.NotEqual(t, -1, target)
	x, y := fileEntryScreenPosition(m, target)

	TeaUpdate(m, clickAt(x, y, tea.MouseLeft))

	assert.Equal(t, target, panel.GetCursor())
}

func TestMouseDoubleClickEntersDirectory(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "child")
	utils.SetupDirectories(t, child)
	m := defaultTestModel(dir)
	panel := m.getFocusedFilePanel()
	target := panel.FindElementIndexByName("child")
	require.NotEqual(t, -1, target)
	x, y := fileEntryScreenPosition(m, target)

	TeaUpdate(m, clickAt(x, y, tea.MouseLeft))
	TeaUpdate(m, clickAt(x, y, tea.MouseLeft))

	assert.Equal(t, child, panel.Location)
}

func TestRightClickOpensContextMenuForTargetRow(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target.txt")
	utils.SetupFiles(t, filepath.Join(dir, "other.txt"), targetPath)
	m := defaultTestModel(dir)
	panel := m.getFocusedFilePanel()
	target := panel.FindElementIndexByName(filepath.Base(targetPath))
	require.NotEqual(t, -1, target)
	x, y := fileEntryScreenPosition(m, target)

	TeaUpdate(m, clickAt(x, y, tea.MouseRight))

	require.True(t, m.contextMenu.open)
	assert.Equal(t, targetPath, m.contextMenu.targetPath)
	assert.Equal(t, target, panel.GetCursor())
}

func TestContextMenuKeyboardCanCopyTarget(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target.txt")
	utils.SetupFiles(t, targetPath)
	m := defaultTestModel(dir)
	x, y := fileEntryScreenPosition(m, 0)
	TeaUpdate(m, clickAt(x, y, tea.MouseRight))

	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyDown})
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.False(t, m.contextMenu.open)
	assert.Equal(t, targetPath, m.clipboard.GetFirstItem())
}

func TestContextMenuOutsideClickDoesNotClickThrough(t *testing.T) {
	dir := t.TempDir()
	utils.SetupFiles(t, filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt"))
	m := defaultTestModel(dir)
	panel := m.getFocusedFilePanel()
	bIndex := panel.FindElementIndexByName("b.txt")
	aIndex := panel.FindElementIndexByName("a.txt")
	bX, bY := fileEntryScreenPosition(m, bIndex)
	TeaUpdate(m, clickAt(bX, bY, tea.MouseRight))
	aX, aY := fileEntryScreenPosition(m, aIndex)

	TeaUpdate(m, clickAt(aX, aY, tea.MouseLeft))

	assert.False(t, m.contextMenu.open)
	assert.Equal(t, bIndex, panel.GetCursor())
}

func TestBreadcrumbClickNavigatesToAncestor(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	grandchild := filepath.Join(child, "grandchild")
	utils.SetupDirectories(t, grandchild)
	m := defaultTestModel(grandchild)
	panel := m.getFocusedFilePanel()
	contentX := -1
	for x := range panel.GetContentWidth() {
		if path, ok := panel.BreadcrumbPathAtContentX(x); ok && path == child {
			contentX = x
			break
		}
	}
	require.NotEqual(t, -1, contentX)
	panelX := m.sidebarModel.GetWidth()

	TeaUpdate(m, clickAt(panelX+1+contentX, 1, tea.MouseLeft))

	assert.Equal(t, child, panel.Location)
}

func TestColumnHeaderClickChangesSort(t *testing.T) {
	originalConfig := common.Config
	config := common.Config
	config.FilePanelExtraColumns = 1
	common.SetConfig(config)
	t.Cleanup(func() { common.SetConfig(originalConfig) })

	dir := t.TempDir()
	utils.SetupFiles(t, filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt"))
	m := defaultTestModel(dir)
	panel := m.getFocusedFilePanel()
	require.True(t, panel.NeedRenderHeaders())
	previousReverse := panel.SortReversed
	panelX := m.sidebarModel.GetWidth()

	TeaUpdate(m, clickAt(panelX+1, 4, tea.MouseLeft))

	assert.NotEqual(t, previousReverse, panel.SortReversed)
}

func TestFooterModeAndSortLabelsAreClickable(t *testing.T) {
	originalConfig := common.Config
	config := common.Config
	config.ShowPanelFooterInfo = true
	common.SetConfig(config)
	t.Cleanup(func() { common.SetConfig(originalConfig) })

	dir := t.TempDir()
	utils.SetupFiles(t, filepath.Join(dir, "file.txt"))
	m := defaultTestModel(dir)
	panel := m.getFocusedFilePanel()
	panelX := m.sidebarModel.GetWidth()
	findActionX := func(want filepanel.FooterAction) int {
		for x := range panel.GetWidth() {
			if action, ok := panel.FooterActionAtX(x); ok && action == want {
				return x
			}
		}
		return -1
	}
	modeX := findActionX(filepanel.FooterActionMode)
	sortX := findActionX(filepanel.FooterActionSort)
	require.NotEqual(t, -1, modeX)
	require.NotEqual(t, -1, sortX)

	TeaUpdate(m, clickAt(panelX+modeX, panel.GetHeight()-1, tea.MouseLeft))
	assert.Equal(t, filepanel.SelectMode, panel.PanelMode)

	TeaUpdate(m, clickAt(panelX+sortX, panel.GetHeight()-1, tea.MouseLeft))
	assert.True(t, m.sortModal.IsOpen())
}

func TestVisibleNotificationReceivesKeysBeforeHiddenContextMenu(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	utils.SetupFiles(t, target)
	m := defaultTestModel(dir)
	x, y := fileEntryScreenPosition(m, 0)
	TeaUpdate(m, clickAt(x, y, tea.MouseRight))
	m.contextMenu.cursor = 1 // Copy
	m.notifyModel = notify.New(true, "Notice", "Foreground modal", notify.NoAction)

	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Empty(t, m.clipboard.GetItems())
	assert.False(t, m.notifyModel.IsOpen())
}

func TestNonRowClickBreaksDoubleClickChain(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "child")
	utils.SetupDirectories(t, child)
	m := defaultTestModel(dir)
	panel := m.getFocusedFilePanel()
	index := panel.FindElementIndexByName("child")
	x, y := fileEntryScreenPosition(m, index)

	TeaUpdate(m, clickAt(x, y, tea.MouseLeft))
	TeaUpdate(m, clickAt(x, 3, tea.MouseLeft)) // search row
	TeaUpdate(m, clickAt(x, y, tea.MouseLeft))

	assert.Equal(t, dir, panel.Location)
}

func TestClickingFileRowBlursSearchInput(t *testing.T) {
	dir := t.TempDir()
	utils.SetupFiles(t, filepath.Join(dir, "file.txt"))
	m := defaultTestModel(dir)
	panel := m.getFocusedFilePanel()
	x, y := fileEntryScreenPosition(m, 0)
	TeaUpdate(m, clickAt(x, 3, tea.MouseLeft))
	require.True(t, panel.SearchBar.Focused())

	TeaUpdate(m, clickAt(x, y, tea.MouseLeft))

	assert.False(t, panel.SearchBar.Focused())
}

func TestFilePanelClicksBlurSidebarSearchBeforeFocusingFile(t *testing.T) {
	for _, test := range []struct {
		name   string
		button tea.MouseButton
	}{
		{name: "left", button: tea.MouseLeft},
		{name: "right", button: tea.MouseRight},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			utils.SetupFiles(t, filepath.Join(dir, "file.txt"))
			m := defaultTestModel(dir)
			m.sidebarModel.SearchBarFocus()
			x, y := fileEntryScreenPosition(m, 0)

			TeaUpdate(m, clickAt(x, y, test.button))

			assert.False(t, m.sidebarModel.SearchBarFocused())
			assert.Equal(t, nonePanelFocus, m.focusPanel)
		})
	}
}

func TestMouseWheelDoesNotScrollBehindOverlayOrContextMenu(t *testing.T) {
	dir := t.TempDir()
	for i := range 100 {
		utils.SetupFiles(t, filepath.Join(dir, fmt.Sprintf("file-%03d.txt", i)))
	}
	m := defaultTestModel(dir)
	panel := m.getFocusedFilePanel()
	before := panel.GetCursor()
	wheel := tea.MouseWheelMsg{Button: tea.MouseWheelDown}

	m.globalSearchModal.Open(dir)
	TeaUpdate(m, wheel)
	assert.Equal(t, before, panel.GetCursor(), "overlay must consume wheel input")
	m.globalSearchModal.Close()

	x, y := fileEntryScreenPosition(m, before)
	m.openContextMenu(tea.Mouse{X: x, Y: y}, 0, panel.GetFocusedItem().Location)
	TeaUpdate(m, wheel)
	assert.Equal(t, before, panel.GetCursor(), "context menu must consume wheel input")
}

func TestRightClickOnUnselectedItemReplacesSelection(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.txt")
	bPath := filepath.Join(dir, "b.txt")
	utils.SetupFiles(t, aPath, bPath)
	m := defaultTestModel(dir)
	panel := m.getFocusedFilePanel()
	panel.ChangeFilePanelMode()
	panel.SetSelected(aPath)
	bIndex := panel.FindElementIndexByLocation(bPath)
	x, y := fileEntryScreenPosition(m, bIndex)

	TeaUpdate(m, clickAt(x, y, tea.MouseRight))

	assert.False(t, panel.CheckSelected(aPath))
	assert.True(t, panel.CheckSelected(bPath))
}

func TestContextMenuBorderClickDoesNotExecuteAction(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	utils.SetupFiles(t, target)
	m := defaultTestModel(dir)
	x, y := fileEntryScreenPosition(m, 0)
	TeaUpdate(m, clickAt(x, y, tea.MouseRight))

	// Copy is the second menu row. Click its left border cell.
	TeaUpdate(m, clickAt(m.contextMenu.x, m.contextMenu.y+2, tea.MouseLeft))

	assert.Empty(t, m.clipboard.GetItems())
}

func TestSidebarDirectoryClickNavigatesAndFocusesSidebar(t *testing.T) {
	root := t.TempDir()
	m := defaultTestModel(root)
	var target string
	var targetY int
	for y := range m.sidebarModel.GetHeight() {
		if path, ok := m.sidebarModel.FocusDirectoryAtY(y, false); ok && path != root {
			target = path
			targetY = y
			break
		}
	}
	require.NotEmpty(t, target)

	TeaUpdate(m, clickAt(1, targetY, tea.MouseLeft))

	assert.Equal(t, target, m.getFocusedFilePanel().Location)
	assert.Equal(t, sidebarFocus, m.focusPanel)
	assert.False(t, m.getFocusedFilePanel().IsFocused)
}

func TestSidebarTitleThenSearchClickFocusesSearchInput(t *testing.T) {
	m := defaultTestModel(t.TempDir())

	TeaUpdate(m, clickAt(1, 1, tea.MouseLeft))
	TeaUpdate(m, clickAt(1, 3, tea.MouseLeft))

	assert.Equal(t, sidebarFocus, m.focusPanel)
	assert.True(t, m.sidebarModel.SearchBarFocused())
}

func TestRightClickFooterDoesNotTargetNonVisibleFile(t *testing.T) {
	dir := t.TempDir()
	for i := range 100 {
		utils.SetupFiles(t, filepath.Join(dir, fmt.Sprintf("file-%03d.txt", i)))
	}
	m := defaultTestModel(dir)
	panel := m.getFocusedFilePanel()
	panelX := m.sidebarModel.GetWidth()

	TeaUpdate(m, clickAt(panelX+1, panel.GetHeight()-1, tea.MouseRight))

	assert.False(t, m.contextMenu.open)
}

func TestRightClickPanelBorderDoesNotTargetFile(t *testing.T) {
	dir := t.TempDir()
	utils.SetupFiles(t, filepath.Join(dir, "file.txt"))
	m := defaultTestModel(dir)
	panelX := m.sidebarModel.GetWidth()
	_, rowY := fileEntryScreenPosition(m, 0)

	TeaUpdate(m, clickAt(panelX, rowY, tea.MouseRight))

	assert.False(t, m.contextMenu.open)
}

func TestLeftClickPanelBorderDoesNotFocusFile(t *testing.T) {
	dir := t.TempDir()
	utils.SetupFiles(t, filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt"))
	m := defaultTestModel(dir)
	panel := m.getFocusedFilePanel()
	bIndex := panel.FindElementIndexByName("b.txt")
	require.NotEqual(t, -1, bIndex)
	panelX := m.sidebarModel.GetWidth()
	_, rowY := fileEntryScreenPosition(m, bIndex)

	TeaUpdate(m, clickAt(panelX, rowY, tea.MouseLeft))

	assert.Equal(t, 0, panel.GetCursor())
}

func TestContextMenuCopyAndCutUseCurrentSelection(t *testing.T) {
	for _, test := range []struct {
		name  string
		item  int
		isCut bool
	}{
		{name: "copy", item: 1, isCut: false},
		{name: "cut", item: 2, isCut: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			aPath := filepath.Join(dir, "a.txt")
			bPath := filepath.Join(dir, "b.txt")
			utils.SetupFiles(t, aPath, bPath)
			m := defaultTestModel(dir)
			panel := m.getFocusedFilePanel()
			panel.ChangeFilePanelMode()
			panel.SetSelected(aPath)
			panel.SetSelected(bPath)
			x, y := fileEntryScreenPosition(m, panel.FindElementIndexByLocation(aPath))
			TeaUpdate(m, clickAt(x, y, tea.MouseRight))
			require.True(t, m.contextMenu.open)
			m.contextMenu.cursor = test.item

			m.executeContextMenuAction()

			assert.Equal(t, []string{aPath, bPath}, m.clipboard.GetItems())
			assert.Equal(t, test.isCut, m.clipboard.IsCut())
		})
	}
}
