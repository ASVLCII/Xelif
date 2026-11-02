package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDirectoryCopyContinuesAfterOriginWorkspaceCloses(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(source, "reviewed.txt"), []byte("retained"), 0o600))
	m := defaultTestModel(source, destination)
	m.ensureWorkspaces()
	completeDirectorySyncPreview(t, m)
	cmd := m.applyPendingDirectorySync()
	require.NotNil(t, cmd)
	_ = m.closeWorkspace()
	m.textViewer.Open("Different workspace", "Keep this view")
	before := m.textViewer.Render()
	msg, ok := cmd().(directorySyncMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)
	m.applyDirectorySync(msg)
	assertDirectorySyncContents(t, filepath.Join(destination, "reviewed.txt"), "retained")
	require.Equal(t, before, m.textViewer.Render())
	require.Nil(t, m.dirSyncCancel)
}

func TestNewDirectoryPreviewDoesNotCancelSubmittedCopy(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(source, "reviewed.txt"), []byte("retained"), 0o600))
	m := defaultTestModel(source, destination)
	completeDirectorySyncPreview(t, m)
	cmd := m.applyPendingDirectorySync()
	require.NotNil(t, cmd)
	completeDirectorySyncPreview(t, m)
	newerApproval := m.pendingSync
	before := m.textViewer.Render()
	msg, ok := cmd().(directorySyncMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)
	m.applyDirectorySync(msg)
	assertDirectorySyncContents(t, filepath.Join(destination, "reviewed.txt"), "retained")
	require.Same(t, newerApproval, m.pendingSync)
	require.Equal(t, before, m.textViewer.Render())
	require.Nil(t, m.dirSyncCancel)
}
