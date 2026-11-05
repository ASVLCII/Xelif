package internal

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func chooseContextAction(t *testing.T, m *model, action int) tea.Cmd {
	t.Helper()
	for row, id := range m.contextMenuActionIDs() {
		if id == action {
			m.contextMenu.cursor = row
			return m.executeContextMenuAction()
		}
	}
	t.Fatalf("context action %d missing from %v", action, m.contextMenuLabels())
	return nil
}

func TestContextBackgroundRightClickAndEmptyKeyboardMenu(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	panel := m.getFocusedFilePanel()
	TeaUpdate(m, clickAt(m.sidebarModel.GetWidth()+2, panel.EntryStartY(), tea.MouseRight))
	require.True(t, m.contextMenu.open)
	require.True(t, m.contextMenu.background)
	require.Contains(t, m.contextMenuActionIDs(), contextNew)
	require.NotContains(t, m.contextMenuActionIDs(), contextTrash)
	require.Contains(t, m.contextMenuLabels(), "Copy folder path")
	m.closeContextMenu()
	m.openKeyboardContextMenu()
	require.True(t, m.contextMenu.background)
	require.True(t, m.contextMenu.open)
}

func TestContextKeyboardMenuRoutesThroughWorkspaceHandler(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyF10, Mod: tea.ModShift})
	require.True(t, m.contextMenu.open)
	require.True(t, m.contextMenu.background)
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	require.False(t, m.contextMenu.open)
}

func TestContextNewFolderMouseAndKeyboardUseSameDialog(t *testing.T) {
	for _, mouse := range []bool{false, true} {
		m := defaultTestModel(t.TempDir())
		m.openContextMenu(tea.Mouse{X: 30, Y: 8}, 0, "")
		chooseContextAction(t, m, contextNew)
		require.True(t, m.contextMenu.open)
		if mouse {
			_, consumed := m.contextMenuClick(tea.Mouse{X: m.contextMenu.x + 2, Y: m.contextMenu.y + 3})
			require.True(t, consumed)
		} else {
			m.contextMenuKey("down")
			m.contextMenuKey("enter")
		}
		require.False(t, m.contextMenu.open)
		require.True(t, m.typingModal.open)
		require.True(t, m.typingModal.directory)
		require.Equal(t, m.getFocusedFilePanel().Location, m.typingModal.location)
	}
}

func TestContextMenuActionsDependOnClickedItem(t *testing.T) {
	require.True(t, contextExtractablePath("archive.tar.bz2"))
	require.False(t, contextExtractablePath("archive.zip.txt"))
	root := t.TempDir()
	folder := filepath.Join(root, "folder")
	archive := filepath.Join(root, "archive.zip")
	plain := filepath.Join(root, "notes.txt")
	utils.SetupDirectories(t, folder)
	utils.SetupFiles(t, archive, plain)
	m := defaultTestModel(root)
	for _, path := range []string{folder, archive, plain} {
		m.openContextMenu(tea.Mouse{}, 0, path)
		ids := m.contextMenuActionIDs()
		require.Equal(t, path == folder, containsContextID(ids, contextNewWorkspace))
		require.Equal(t, path == folder, containsContextID(ids, contextFavourite))
		require.Equal(t, path != folder, containsContextID(ids, contextEditor))
		chooseContextAction(t, m, contextArchives)
		require.Equal(t, path == archive, containsContextID(m.contextMenuActionIDs(), contextExtract))
	}
}

func containsContextID(ids []int, wanted int) bool {
	for _, id := range ids {
		if id == wanted {
			return true
		}
	}
	return false
}

func TestContextMenuSubmenusConsumeEscapeAndKeepParentRow(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	m.openContextMenu(tea.Mouse{}, 0, "")
	chooseContextAction(t, m, contextView)
	parent := m.contextMenu.parentCursor
	m.contextMenuKey("esc")
	require.True(t, m.contextMenu.open)
	require.Zero(t, m.contextMenu.page)
	require.Equal(t, parent, m.contextMenu.cursor)
	m.contextMenuKey("esc")
	require.False(t, m.contextMenu.open)
	require.False(t, m.typingModal.open)
}

func TestContextMenuBackgroundRejectsChangedDirectory(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	m.openContextMenu(tea.Mouse{}, 0, "")
	chooseContextAction(t, m, contextNew)
	m.getFocusedFilePanel().Location = t.TempDir()
	chooseContextAction(t, m, contextNewFile)
	require.False(t, m.typingModal.open)
	require.True(t, m.notifyModel.IsOpen())
	require.Contains(t, m.notifyModel.GetContent(), "folder changed")
}

func TestContextMenuEmptyPasteDoesNotDismissOrMutate(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	m.openContextMenu(tea.Mouse{}, 0, "")
	require.Nil(t, chooseContextAction(t, m, contextPaste))
	require.True(t, m.contextMenu.open)
	require.Contains(t, m.contextMenuLabels(), "Paste (clipboard empty)")
}

func TestContextMenuPropertiesUsesListedSnapshot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0o600))
	m := defaultTestModel(root)
	m.openContextMenu(tea.Mouse{}, 0, path)
	chooseContextAction(t, m, contextProperties)
	require.True(t, m.notifyModel.IsOpen())
	require.Equal(t, "Properties", m.notifyModel.GetTitle())
	require.Contains(t, m.notifyModel.GetContent(), "5 B")
	require.Contains(t, m.notifyModel.GetContent(), "Listed metadata")
}

func TestContextMenuAllPagesFitReferenceSizes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "archive.zip")
	utils.SetupFiles(t, path)
	m := defaultTestModel(root)
	for _, size := range [][2]int{{60, 24}, {80, 24}, {120, 36}, {180, 50}} {
		m.fullWidth, m.fullHeight = size[0], size[1]
		for _, target := range []string{"", path} {
			for _, page := range []int{0, contextNew, contextArchives, contextView, contextMore} {
				m.openContextMenu(tea.Mouse{X: size[0] - 1, Y: size[1] - 1}, 0, target)
				m.contextMenu.page = page
				m.clampContextMenu()
				view := m.contextMenuRender()
				require.LessOrEqual(t, m.contextMenu.x+lipgloss.Width(view), size[0])
				require.LessOrEqual(t, m.contextMenu.y+lipgloss.Height(view), size[1])
			}
		}
	}
}

func TestContextRefreshRetainsHiddenFilePreference(t *testing.T) {
	root := t.TempDir()
	utils.SetupFiles(t, filepath.Join(root, ".hidden"), filepath.Join(root, "visible.txt"))
	m := defaultTestModel(root)
	m.fileModel.DisplayDotFiles = false
	m.getFocusedFilePanel().UpdateElementsIfNeeded(true, false)
	m.openContextMenu(tea.Mouse{}, 0, "")
	chooseContextAction(t, m, contextRefresh)
	require.Equal(t, -1, m.getFocusedFilePanel().FindElementIndexByName(".hidden"))
}

func TestContextPropertiesDialogFitsNarrowWindowAndMouseOkay(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	utils.SetupFiles(t, path)
	m := defaultTestModel(root)
	m.fullWidth, m.fullHeight = 60, 24
	m.openContextMenu(tea.Mouse{}, 0, path)
	chooseContextAction(t, m, contextProperties)
	rendered := m.notificationRender()
	require.LessOrEqual(t, lipgloss.Width(rendered), m.fullWidth)
	require.LessOrEqual(t, lipgloss.Height(rendered), m.fullHeight)
	x, y := renderedLabelPosition(t, rendered, common.ModalOkayInputText)
	x += m.fullWidth/2 - lipgloss.Width(rendered)/2
	y += m.fullHeight/2 - lipgloss.Height(rendered)/2
	TeaUpdate(m, clickAt(x, y, tea.MouseLeft))
	require.False(t, m.notifyModel.IsOpen())
}

func TestContextNewFolderCollisionDoesNotReportSuccess(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "existing")
	utils.SetupDirectories(t, folder)
	marker := filepath.Join(folder, "keep.txt")
	require.NoError(t, os.WriteFile(marker, []byte("keep"), 0o600))
	err := createItem(root, "existing"+string(filepath.Separator))
	require.ErrorIs(t, err, os.ErrExist)
	data, readErr := os.ReadFile(marker)
	require.NoError(t, readErr)
	require.Equal(t, "keep", string(data))
}

func TestContextKeyboardAndMouseNormalizeUnselectedTarget(t *testing.T) {
	for _, mouse := range []bool{false, true} {
		root := t.TempDir()
		a, b := filepath.Join(root, "a.txt"), filepath.Join(root, "b.txt")
		utils.SetupFiles(t, a, b)
		m := dailyDriverTestModel(root)
		panel := m.getFocusedFilePanel()
		panel.PanelMode = filepanel.SelectMode
		panel.SetSelected(a)
		index := panel.FindElementIndexByLocation(b)
		require.True(t, panel.FocusIndex(index))
		if mouse {
			x, y := fileEntryScreenPosition(m, index)
			TeaUpdate(m, clickAt(x, y, tea.MouseRight))
		} else {
			TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyF10, Mod: tea.ModShift})
		}
		require.True(t, m.contextMenu.open)
		require.False(t, panel.CheckSelected(a))
		require.True(t, panel.CheckSelected(b))
		chooseContextAction(t, m, contextCopy)
		require.Equal(t, []string{b}, m.clipboard.GetItems())
	}
}

func TestContextCustomLeftBindingKeepsPrecedence(t *testing.T) {
	previous := common.Hotkeys.ListUp
	t.Cleanup(func() { common.Hotkeys.ListUp = previous })
	common.Hotkeys.ListUp = []string{"left"}
	m := defaultTestModel(t.TempDir())
	m.openContextMenu(tea.Mouse{}, 0, "")
	chooseContextAction(t, m, contextView)
	m.contextMenu.cursor = 2
	m.contextMenuKey("left")
	require.Equal(t, contextView, m.contextMenu.page)
	require.Equal(t, 1, m.contextMenu.cursor)
}

func TestContextResizeKeepsRenderedAndMouseGeometryTogether(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	for _, page := range []int{0, contextView} {
		m.fullWidth, m.fullHeight = 120, 36
		m.openContextMenu(tea.Mouse{X: 119, Y: 35}, 0, "")
		m.contextMenu.page = page
		_ = m.handleWindowResize(tea.WindowSizeMsg{Width: 60, Height: 24})
		width, height := m.contextMenuDimensions()
		require.LessOrEqual(t, m.contextMenu.x+width, 60)
		require.LessOrEqual(t, m.contextMenu.y+height, 24)
		_, consumed := m.contextMenuClick(tea.Mouse{X: m.contextMenu.x + 2, Y: m.contextMenu.y + 2})
		require.True(t, consumed)
	}
}
