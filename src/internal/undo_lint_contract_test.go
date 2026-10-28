package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteUndoCompletedJournalIsNilWithoutError(t *testing.T) {
	directory := t.TempDir()
	original := filepath.Join(directory, "original.txt")
	moved := filepath.Join(directory, "moved.txt")
	require.NoError(t, os.WriteFile(moved, []byte("preserved content"), 0600))
	m := &model{}
	m.recordUndo("move", []pathMove{{from: original, to: moved}})
	action := m.lastUndo

	remaining, err := executeUndo(action)

	require.NoError(t, err)
	assert.Nil(t, remaining, "successful completion clears the remaining undo journal")
	content, err := os.ReadFile(original)
	require.NoError(t, err)
	assert.Equal(t, "preserved content", string(content))
	assert.NoFileExists(t, moved)
	assert.Len(t, action.moves, 1, "execution must not mutate the caller's journal")
}

func TestExecuteUndoEmptyJournalIsAlreadyComplete(t *testing.T) {
	remaining, err := executeUndo(&undoAction{name: "completed"})
	require.NoError(t, err)
	assert.Nil(t, remaining)
}

func TestExecuteUndoMissingJournalStillReturnsError(t *testing.T) {
	remaining, err := executeUndo(nil)
	require.EqualError(t, err, "nothing to undo")
	assert.Nil(t, remaining)
}

func TestCopyElementNoReplaceDirectoryContract(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	destination := filepath.Join(directory, "destination")
	child := filepath.Join("nested", "content.txt")
	require.NoError(t, os.MkdirAll(filepath.Join(source, "nested"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(source, child), []byte("source content"), 0600))

	require.NoError(t, copyElementNoReplace(source, destination))
	content, err := os.ReadFile(filepath.Join(destination, child))
	require.NoError(t, err)
	assert.Equal(t, "source content", string(content))
	assert.FileExists(t, filepath.Join(source, child), "copy leaves the original tree intact")

	require.NoError(t, os.WriteFile(filepath.Join(destination, child), []byte("existing content"), 0600))
	require.Error(t, copyElementNoReplace(source, destination))
	content, err = os.ReadFile(filepath.Join(destination, child))
	require.NoError(t, err)
	assert.Equal(t, "existing content", string(content), "existing destination is not replaced or removed")
}
