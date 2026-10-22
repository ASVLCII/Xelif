package internal

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/ui/archiveviewer"
	"github.com/yorukot/superfile/src/internal/ui/processbar"
)

func TestRefreshTickUpdatesFolderWithoutWatcher(t *testing.T) {
	root := t.TempDir()
	m := defaultTestModel(root)
	m.disableMetadata = true
	panel := m.getFocusedFilePanel()
	require.NoError(t, os.WriteFile(filepath.Join(root, "external.txt"), nil, 0o600))
	panel.LastTimeGetElement = time.Now().Add(-time.Hour)
	_, cmd := m.Update(refreshTickMsg{})
	require.NotEqual(t, -1, panel.FindElementIndexByName("external.txt"))
	require.NotNil(t, cmd)
	// Inspect the batch without starting unrelated preview/metadata/listener
	// commands. Only the refresh tick is expected for this idle model.
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		require.IsType(t, refreshTickMsg{}, msg)
		return
	}
	require.NotEmpty(t, batch)
}

func TestCompletedFileOperationsBypassRefreshThrottle(t *testing.T) {
	for _, test := range []struct {
		name string
		msg  tea.Msg
	}{
		{"create", NewCreateOperationMsg(processbar.Successful, 0)},
		{"delete", NewDeleteOperationMsg(processbar.Successful, 0)},
		{"paste", NewPasteOperationMsg(processbar.Successful, 0)},
		{"undo", UndoOperationMsg{}},
		{"compress", NewCompressOperationMsg(processbar.Successful, 0)},
		{"extract", NewExtractOperationMsg(processbar.Successful, 0)},
		{"partial-directory-sync", directorySyncMsg{err: errors.New("partial copy")}},
		{"partial-batch-rename", batchRenameOperationMsg{err: errors.New("partial rename")}},
		{"partial-metadata", metadataOperationMsg{err: errors.New("partial metadata")}},
		{"partial-archive", archiveOperationMsg{kind: archiveviewer.ExtractAction, err: errors.New("partial extraction")}},
		{"partial-download", remoteOperationMsg{kind: remoteDownloadFile, err: errors.New("partial download")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			m := defaultTestModel(root)
			m.disableMetadata = true
			panel := m.getFocusedFilePanel()
			panel.LastTimeGetElement = time.Now().Add(time.Hour)
			require.NoError(t, os.WriteFile(filepath.Join(root, "completed.txt"), nil, 0o600))
			m.Update(test.msg)
			require.NotEqual(t, -1, panel.FindElementIndexByName("completed.txt"))
		})
	}
}

func TestRenameBypassesRefreshThrottleWithoutWatcher(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "before.txt"), nil, 0o600))
	m := defaultTestModel(root)
	panel := m.getFocusedFilePanel()
	panel.LastTimeGetElement = time.Now().Add(time.Hour)
	m.panelItemRename()
	panel.Rename.SetValue("after.txt")
	m.confirmRename()
	require.Equal(t, -1, panel.FindElementIndexByName("before.txt"))
	require.NotEqual(t, -1, panel.FindElementIndexByName("after.txt"))
}
