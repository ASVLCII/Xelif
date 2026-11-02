package internal

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/session"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
)

func dailyDriverTestModel(path string) *model {
	m := defaultTestModel(path)
	m.ensureWorkspaces()
	_ = m.handleWindowResize(tea.WindowSizeMsg{Width: DefaultTestModelWidth, Height: DefaultTestModelHeight})
	return m
}

func TestWorkspaceNavigationSelectionAndGlobals(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.go"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), nil, 0o600))
	}
	m := dailyDriverTestModel(root)
	m.ensureWorkspaces()
	p := m.getFocusedFilePanel()
	p.SearchBar.SetValue(".txt")
	p.UpdateElementsIfNeeded(true, false)
	p.SetCursorPosition(1)
	p.PanelMode = filepanel.SelectMode
	p.SetSelected(filepath.Join(root, "b.txt"))
	tasks := &m.processBarModel
	_ = m.newWorkspace(other)
	require.Len(t, m.workspaces, 2)
	require.Equal(t, other, m.getFocusedFilePanel().Location)
	require.Zero(t, m.getFocusedFilePanel().SelectedCount())
	require.False(t, m.fileModel.FilePreview.IsOpen())
	_ = m.activateWorkspace(0)
	require.Equal(t, root, m.getFocusedFilePanel().Location)
	require.Equal(t, ".txt", m.getFocusedFilePanel().SearchBar.Value())
	require.Equal(t, "b.txt", m.getFocusedFilePanel().GetFocusedItem().Name)
	require.Equal(t, uint(1), m.getFocusedFilePanel().SelectedCount())
	require.Same(t, tasks, m.fileModel.Tasks)
}

func TestWorkspaceCloseReopenReorderAndDirtyGuard(t *testing.T) {
	root := t.TempDir()
	m := dailyDriverTestModel(root)
	original := m.workspaces[0].State.ID
	_ = m.newWorkspace(root)
	second := m.workspaces[1].State.ID
	require.NotEqual(t, m.workspaces[0].State.Title, m.workspaces[1].State.Title)
	m.moveWorkspace(-1)
	require.Equal(t, second, m.workspaces[m.activeWorkspace].State.ID)
	m.fileModel.Renaming = true
	_ = m.activateWorkspace(1)
	require.Equal(t, 0, m.activeWorkspace)
	m.fileModel.Renaming = false
	_ = m.closeWorkspace()
	require.Equal(t, original, m.workspaces[m.activeWorkspace].State.ID)
	_ = m.reopenWorkspace()
	require.Equal(t, second, m.workspaces[m.activeWorkspace].State.ID)
	_ = m.closeWorkspace()
	_ = m.closeWorkspace()
	require.Len(t, m.workspaces, 1)
	require.Equal(t, root, m.getFocusedFilePanel().Location)
	require.Equal(t, notQuitting, m.modelQuitState)
}

func TestWorkspaceSessionRestartDoesNotRestoreSelections(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "file.txt"), nil, 0o600))
	m := dailyDriverTestModel(root)
	m.sessionStore = session.NewStore(filepath.Join(t.TempDir(), "session.json"), t.TempDir())
	m.getFocusedFilePanel().SetSelected(filepath.Join(root, "file.txt"))
	_ = m.newWorkspace(t.TempDir())
	m.workspaces[1].State.Title = "Working files"
	require.NoError(t, m.saveLastSession())
	n := dailyDriverTestModel(root)
	n.sessionStore = m.sessionStore
	require.NoError(t, n.restoreLastSession())
	require.Len(t, n.workspaces, 2)
	require.Equal(t, "Working files", n.workspaces[n.activeWorkspace].State.Title)
	_ = n.activateWorkspace(0)
	require.Zero(t, n.getFocusedFilePanel().SelectedCount())
}

func TestWorkspaceRowsAndControlsAtReferenceDimensions(t *testing.T) {
	for _, size := range [][2]int{{60, 24}, {80, 24}, {120, 36}, {180, 50}} {
		m := dailyDriverTestModel(t.TempDir())
		_ = m.handleWindowResize(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		require.NoError(t, m.validateLayout())
		require.NoError(t, validateRender(m.mainComponentsRender(), size[1], size[0], false))
	}
}

func TestWorkspaceCustomShortcutPrecedence(t *testing.T) {
	type shortcutSet struct{ NewFileTab []string }
	require.True(t, workspaceShortcutConflict("ctrl+t", reflect.ValueOf(shortcutSet{NewFileTab: []string{"ctrl+t"}})))
	require.False(t, workspaceShortcutConflict("ctrl+t", reflect.ValueOf(shortcutSet{})))
}

func TestWorkspaceRejectsOldAsyncReplyAfterReopen(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	cmd := m.workspaceCommand(func() tea.Msg { return tea.KeyPressMsg{Code: 'q', Text: "q"} })
	reply := cmd()
	_ = m.newWorkspace(t.TempDir())
	_ = m.activateWorkspace(0)
	_, next := m.Update(reply)
	require.Nil(t, next)
	require.Equal(t, notQuitting, m.modelQuitState)
}

func TestWorkspaceResponsiveSplitRestoresUnderlyingPanels(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	_, err := m.fileModel.CreateNewFilePanel(t.TempDir())
	require.NoError(t, err)
	require.True(t, m.getFocusedFilePanel().WorkspaceChrome)
	for _, width := range []int{60, 80, 120, 180, 60, 180} {
		_ = m.handleWindowResize(tea.WindowSizeMsg{Width: width, Height: 24})
		require.Len(t, m.fileModel.FilePanels, 2)
		if width < 100 {
			require.Len(t, m.fileModel.VisiblePanelIndices(), 1)
		}
		require.NoError(t, m.validateLayout())
		require.NoError(t, validateRender(m.mainComponentsRender(), 24, width, false))
	}
}

func workspaceTestIDs(m *model) []string {
	ids := make([]string, len(m.workspaces))
	for i, w := range m.workspaces {
		ids[i] = w.State.ID
	}
	return ids
}

func TestWorkspaceContextCloseInactiveKeepsActiveFormsAndOperations(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	m := dailyDriverTestModel(root)
	closedID := m.workspaces[0].State.ID
	_ = m.newWorkspace(other)
	activeID := m.workspaces[m.activeWorkspace].State.ID
	require.NotEqual(t, closedID, activeID)

	m.getFocusedFilePanel().SearchBar.SetValue("draft")
	m.getFocusedFilePanel().SearchBar.Focus()
	m.fileModel.Renaming = true
	m.operations.revision = 77
	m.operations.selected = "op-1"
	tasks := m.fileModel.Tasks

	cmd := m.closeWorkspaceByID(closedID)
	require.Nil(t, cmd)
	require.Len(t, m.workspaces, 1)
	require.Equal(t, activeID, m.workspaces[m.activeWorkspace].State.ID)
	require.Equal(t, other, m.getFocusedFilePanel().Location)
	require.Equal(t, "draft", m.getFocusedFilePanel().SearchBar.Value())
	require.True(t, m.getFocusedFilePanel().SearchBar.Focused())
	require.True(t, m.fileModel.Renaming)
	require.Equal(t, uint64(77), m.operations.revision)
	require.Equal(t, "op-1", m.operations.selected)
	require.Same(t, tasks, m.fileModel.Tasks)
	require.Len(t, m.closedWorkspaces, 1)
	require.Equal(t, closedID, m.closedWorkspaces[0].State.ID)

	// A dirty active form still blocks closing the active workspace itself.
	require.Nil(t, m.closeWorkspaceByID(activeID))
	require.Len(t, m.workspaces, 1)
	require.True(t, m.fileModel.Renaming)
}

func TestWorkspaceContextCloseActiveUsesLeftNeighborAndFreshLaunch(t *testing.T) {
	root := t.TempDir()
	m := dailyDriverTestModel(root)
	rootID := m.workspaces[0].State.ID
	left := t.TempDir()
	_ = m.newWorkspace(left)
	leftID := m.workspaces[m.activeWorkspace].State.ID
	right := t.TempDir()
	_ = m.newWorkspace(right)
	rightID := m.workspaces[m.activeWorkspace].State.ID

	_ = m.closeWorkspaceByID(rightID)
	require.Equal(t, leftID, m.workspaces[m.activeWorkspace].State.ID)
	require.Equal(t, left, m.getFocusedFilePanel().Location)

	_ = m.closeWorkspaceByID(leftID)
	require.Equal(t, rootID, m.workspaces[m.activeWorkspace].State.ID)
	require.Equal(t, root, m.getFocusedFilePanel().Location)

	_ = m.closeWorkspaceByID(rootID)
	require.Len(t, m.workspaces, 1)
	require.Equal(t, 0, m.activeWorkspace)
	require.Equal(t, root, m.getFocusedFilePanel().Location)
	require.Equal(t, root, m.launchDirectory)
	require.NotEqual(t, rootID, m.workspaces[0].State.ID)
}

func TestWorkspaceContextReorderInactiveKeepsActive(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	ids := []string{m.workspaces[0].State.ID}
	for range 2 {
		_ = m.newWorkspace(t.TempDir())
		ids = append(ids, m.workspaces[m.activeWorkspace].State.ID)
	}
	require.Equal(t, ids[2], m.workspaces[m.activeWorkspace].State.ID)

	m.openWorkspaceContextMenu(tea.Mouse{X: 1, Y: 0}, ids[1])
	require.True(t, m.contextMenu.open)
	require.Equal(t, ids[1], m.contextMenu.workspaceTargetID)
	require.Equal(t, ids[2], m.workspaces[m.activeWorkspace].State.ID)

	m.contextMenu.cursor = 2 // Move workspace left.
	_ = m.executeContextMenuAction()
	require.Equal(t, []string{ids[1], ids[0], ids[2]}, workspaceTestIDs(m))
	require.Equal(t, ids[2], m.workspaces[m.activeWorkspace].State.ID)

	m.contextMenu.open = true
	m.contextMenu.workspaceTarget = true
	m.contextMenu.workspaceTargetID = ids[2]
	m.contextMenu.cursor = 2 // Move the active workspace left.
	_ = m.executeContextMenuAction()
	require.Equal(t, []string{ids[1], ids[2], ids[0]}, workspaceTestIDs(m))
	require.Equal(t, ids[2], m.workspaces[m.activeWorkspace].State.ID)
}

func TestWorkspaceContextStaleTargetDoesNotActOnNeighbor(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	ids := []string{m.workspaces[0].State.ID}
	for range 2 {
		_ = m.newWorkspace(t.TempDir())
		ids = append(ids, m.workspaces[m.activeWorkspace].State.ID)
	}

	m.openWorkspaceContextMenu(tea.Mouse{X: 1, Y: 0}, ids[1])
	require.Nil(t, m.closeWorkspaceByID(ids[1]))
	before := workspaceTestIDs(m)
	m.contextMenu.cursor = 1 // Close the now-stale target.
	require.Nil(t, m.executeContextMenuAction())
	require.Equal(t, before, workspaceTestIDs(m))
	require.Equal(t, ids[2], m.workspaces[m.activeWorkspace].State.ID)
	require.Contains(t, m.workspaceNotice, "no longer open")

	m.openWorkspaceContextMenu(tea.Mouse{X: 1, Y: 0}, ids[0])
	require.True(t, m.contextMenu.open)
	_ = m.closeWorkspaceByID(ids[0])
	m.contextMenu.cursor = 0 // Rename the now-stale target.
	require.Nil(t, m.executeContextMenuAction())
	require.False(t, m.workspaceRenaming)
	require.Equal(t, []string{ids[2]}, workspaceTestIDs(m))
}

func TestWorkspaceContextRenameInactivePreservesGraphemes(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	ids := []string{m.workspaces[0].State.ID}
	for range 2 {
		_ = m.newWorkspace(t.TempDir())
		ids = append(ids, m.workspaces[m.activeWorkspace].State.ID)
	}
	title := "Проект 😀 e\u0301"
	m.getFocusedFilePanel().SearchBar.SetValue("keep")

	m.openWorkspaceContextMenu(tea.Mouse{X: 1, Y: 0}, ids[1])
	m.contextMenu.cursor = 0 // Rename workspace.
	require.NotNil(t, m.executeContextMenuAction())
	require.True(t, m.workspaceRenaming)
	require.Equal(t, ids[1], m.workspaceRenameID)
	require.Equal(t, ids[2], m.workspaces[m.activeWorkspace].State.ID)

	m.workspaceNameInput.SetValue(title)
	_ = m.workspaceRenameUpdate(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.False(t, m.workspaceRenaming)
	index := m.workspaceIndexByID(ids[1])
	require.GreaterOrEqual(t, index, 0)
	require.Equal(t, title, m.workspaces[index].State.Title)
	require.Equal(t, ids[2], m.workspaces[m.activeWorkspace].State.ID)
	require.Equal(t, "keep", m.getFocusedFilePanel().SearchBar.Value())

	_ = m.closeWorkspaceByID(ids[1])
	require.Len(t, m.closedWorkspaces, 1)
	require.Equal(t, title, m.closedWorkspaces[0].State.Title)
	_ = m.reopenWorkspace()
	require.Equal(t, ids[1], m.workspaces[m.activeWorkspace].State.ID)
	require.Equal(t, title, m.workspaces[m.activeWorkspace].State.Title)
}

func TestWorkspaceActiveRenameWaitsForDirtyForm(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	activeID := m.workspaces[m.activeWorkspace].State.ID
	m.fileModel.Renaming = true
	require.Nil(t, m.beginWorkspaceRename(activeID))
	require.False(t, m.workspaceRenaming)

	m.fileModel.Renaming = false
	require.NotNil(t, m.beginWorkspaceRename(activeID))
	require.True(t, m.workspaceRenaming)
	require.Equal(t, activeID, m.workspaceRenameID)
	title := "Active 😀 e\u0301"
	m.workspaceNameInput.SetValue(title)
	_ = m.workspaceRenameUpdate(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.False(t, m.workspaceRenaming)
	require.Equal(t, title, m.workspaces[m.activeWorkspace].State.Title)
	require.Equal(t, activeID, m.workspaces[m.activeWorkspace].State.ID)
}

func TestWorkspaceClosedHistoryBoundedAndSnapshotsIndependent(t *testing.T) {
	root := t.TempDir()
	m := dailyDriverTestModel(root)
	for range maxClosedWorkspaces + 2 {
		_ = m.newWorkspace(t.TempDir())
		_ = m.closeWorkspaceByID(m.workspaces[m.activeWorkspace].State.ID)
	}
	for len(m.workspaces) > 1 {
		_ = m.closeWorkspaceByID(m.workspaces[m.activeWorkspace].State.ID)
	}
	_ = m.closeWorkspaceByID(m.workspaces[m.activeWorkspace].State.ID)
	require.Len(t, m.closedWorkspaces, maxClosedWorkspaces)
	require.Len(t, m.workspaces, 1)
	require.Equal(t, root, m.getFocusedFilePanel().Location)

	m.getFocusedFilePanel().UpdateElementsIfNeeded(true, false)
	view := m.getFocusedFilePanel().SnapshotView()
	if view.Selected == nil {
		view.Selected = map[string]os.FileInfo{}
	}
	view.Selected["ghost"] = nil
	if view.SelectionOrder == nil {
		view.SelectionOrder = map[string]int{}
	}
	view.SelectionOrder["ghost"] = 0
	w := workspaceRuntime{Views: []filepanel.ViewState{view}}
	clone := cloneWorkspaceRuntime(w)
	clone.Views[0].Selected["extra"] = nil
	clone.Views[0].SelectionOrder["extra"] = 1
	require.NotContains(t, w.Views[0].Selected, "extra")
	require.NotContains(t, w.Views[0].SelectionOrder, "extra")
}
