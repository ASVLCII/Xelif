package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
)

func TestBatchRenameRequiresPreviewThenSupportsUndo(t *testing.T) {
	directory := t.TempDir()
	first := filepath.Join(directory, "first.txt")
	second := filepath.Join(directory, "second.txt")
	require.NoError(t, os.WriteFile(first, []byte("first"), 0o600))
	require.NoError(t, os.WriteFile(second, []byte("second"), 0o600))
	m := defaultTestModel(directory)
	panel := m.getFocusedFilePanel()
	panel.ChangeFilePanelMode()
	panel.SetSelectedAll([]string{first, second})

	message, cmd, err := m.executeBatchRenameAction(common.BatchRenameAction{
		Operation: common.BatchRenamePrefix, Replacement: "done-",
	})
	require.NoError(t, err)
	assert.Nil(t, cmd)
	assert.Contains(t, message, "Previewed 2")
	assert.NotNil(t, m.pendingBatchRename)
	_, err = os.Stat(first)
	require.NoError(t, err, "preview must not mutate")

	_, cmd, err = m.executeBatchRenameAction(common.BatchRenameAction{Operation: common.BatchRenameApply})
	require.NoError(t, err)
	require.NotNil(t, cmd)
	msg := cmd().(batchRenameOperationMsg)
	m.applyBatchRenameOperation(msg)

	for _, name := range []string{"done-first.txt", "done-second.txt"} {
		_, statErr := os.Stat(filepath.Join(directory, name))
		require.NoError(t, statErr)
	}
	assert.Nil(t, m.pendingBatchRename)
	require.NoError(t, m.undoLastFileOperation())
	for _, path := range []string{first, second} {
		_, statErr := os.Stat(path)
		require.NoError(t, statErr)
	}
}

func TestBatchRenameApplyWithoutPreviewFails(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	_, cmd, err := m.executeBatchRenameAction(common.BatchRenameAction{Operation: common.BatchRenameApply})
	require.ErrorContains(t, err, "no batch rename preview")
	assert.Nil(t, cmd)
}
