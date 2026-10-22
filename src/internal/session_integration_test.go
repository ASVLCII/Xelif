package internal

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/session"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func testSessionStore(t *testing.T) *session.Store {
	t.Helper()
	root := t.TempDir()
	return session.NewStore(filepath.Join(root, "session.json"), filepath.Join(root, "workspaces"))
}

func TestSessionRoundTripRestoresPanelsTabsAndLayout(t *testing.T) {
	root := t.TempDir()
	tab := filepath.Join(root, "tab")
	secondPanel := filepath.Join(root, "second")
	utils.SetupDirectories(t, tab, secondPanel)
	m := defaultTestModel(root)
	require.NoError(t, m.getFocusedFilePanel().NewTab(tab))
	_, err := m.createNewFilePanel(secondPanel)
	require.NoError(t, err)
	m.fileModel.DisplayDotFiles = true
	m.toggleFooter = true
	m.fileModel.FilePreview.Close()
	m.sessionStore = testSessionStore(t)

	require.NoError(t, m.saveLastSession())
	restored := defaultTestModel(root)
	restored.sessionStore = m.sessionStore
	require.NoError(t, restored.restoreLastSession())

	require.Len(t, restored.fileModel.FilePanels, 2)
	assert.Equal(t, 1, restored.fileModel.FocusedPanelIndex)
	assert.Equal(t, 2, restored.fileModel.FilePanels[0].TabCount())
	assert.Equal(t, 1, restored.fileModel.FilePanels[0].ActiveTabIndex())
	assert.Equal(t, tab, restored.fileModel.FilePanels[0].Location)
	assert.Equal(t, secondPanel, restored.fileModel.FilePanels[1].Location)
	assert.True(t, restored.fileModel.DisplayDotFiles)
	assert.True(t, restored.toggleFooter)
	assert.False(t, restored.fileModel.FilePreview.IsOpen())
}

func TestMissingLastSessionKeepsFallbackPanel(t *testing.T) {
	fallback := t.TempDir()
	m := defaultTestModel(fallback)
	m.sessionStore = testSessionStore(t)

	require.NoError(t, m.restoreLastSession())

	assert.Equal(t, fallback, m.getFocusedFilePanel().Location)
}

func TestNamedWorkspaceLifecycleAppliesSavedState(t *testing.T) {
	root := t.TempDir()
	saved := filepath.Join(root, "saved")
	other := filepath.Join(root, "other")
	utils.SetupDirectories(t, saved, other)
	m := defaultTestModel(saved)
	m.sessionStore = testSessionStore(t)

	require.NoError(t, m.saveWorkspace("project"))
	require.NoError(t, m.updateCurrentFilePanelDir(other))
	require.NoError(t, m.loadWorkspace("project"))

	assert.Equal(t, saved, m.getFocusedFilePanel().Location)
	names, err := m.sessionStore.ListWorkspaces()
	require.NoError(t, err)
	assert.Equal(t, []string{"project"}, names)
	require.NoError(t, m.deleteWorkspace("project"))
}
