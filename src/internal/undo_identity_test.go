package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUndoRejectsReplacedMovedItemBeforeMovingAnyFiles(t *testing.T) {
	root := t.TempDir()
	beforeA, afterA := filepath.Join(root, "before-a"), filepath.Join(root, "after-a")
	beforeB, afterB := filepath.Join(root, "before-b"), filepath.Join(root, "after-b")
	require.NoError(t, os.WriteFile(afterA, []byte("original a"), 0600))
	require.NoError(t, os.WriteFile(afterB, []byte("original b"), 0600))
	m := &model{}
	m.recordUndo("move", []pathMove{{from: beforeA, to: afterA}, {from: beforeB, to: afterB}})
	// Move the original aside to avoid filesystem ID reuse in the regression.
	preserved := filepath.Join(root, "preserved-original")
	require.NoError(t, os.Rename(afterA, preserved))
	require.NoError(t, os.WriteFile(afterA, []byte("replacement"), 0600))

	require.ErrorContains(t, m.undoLastFileOperation(), "changed")
	require.NoFileExists(t, beforeA)
	require.NoFileExists(t, beforeB)
	require.FileExists(t, afterB, "preflight must reject before undoing another entry")
	data, err := os.ReadFile(afterA)
	require.NoError(t, err)
	require.Equal(t, "replacement", string(data))
	data, err = os.ReadFile(preserved)
	require.NoError(t, err)
	require.Equal(t, "original a", string(data))
	require.NotNil(t, m.lastUndo, "failed undo keeps the journal")
}

func TestUndoRejectsReplacedMovedDirectory(t *testing.T) {
	root := t.TempDir()
	before, after := filepath.Join(root, "before"), filepath.Join(root, "after")
	require.NoError(t, os.Mkdir(after, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(after, "original.txt"), []byte("original"), 0600))
	m := &model{}
	m.recordUndo("move", []pathMove{{from: before, to: after}})
	require.NoError(t, os.Rename(after, filepath.Join(root, "preserved")))
	require.NoError(t, os.Mkdir(after, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(after, "replacement.txt"), []byte("replacement"), 0600))
	require.ErrorContains(t, m.undoLastFileOperation(), "changed")
	require.FileExists(t, filepath.Join(after, "replacement.txt"))
	require.NoDirExists(t, before)
}

func TestUndoRetryRetainsOriginalIdentity(t *testing.T) {
	root := t.TempDir()
	before, after := filepath.Join(root, "before"), filepath.Join(root, "after")
	preserved := filepath.Join(root, "preserved")
	require.NoError(t, os.WriteFile(after, []byte("original"), 0600))
	m := &model{}
	m.recordUndo("move", []pathMove{{from: before, to: after}})
	queued := cloneUndoAction(m.lastUndo)
	require.NoError(t, os.Rename(after, preserved))
	require.NoError(t, os.WriteFile(after, []byte("replacement"), 0600))
	remaining, err := executeUndo(queued)
	require.ErrorContains(t, err, "changed")
	require.Len(t, remaining.identities, 1)
	// Retrying cannot bless the replacement by refreshing its fingerprint.
	_, err = executeUndo(remaining)
	require.ErrorContains(t, err, "changed")
	require.NoError(t, os.Rename(after, filepath.Join(root, "replacement-aside")))
	require.NoError(t, os.Rename(preserved, after))
	completed, err := executeUndo(remaining)
	require.NoError(t, err)
	require.Nil(t, completed)
	data, err := os.ReadFile(before)
	require.NoError(t, err)
	require.Equal(t, "original", string(data))
}

func TestUndoAllowsContentEditsToTheSameMovedItem(t *testing.T) {
	root := t.TempDir()
	before, after := filepath.Join(root, "before"), filepath.Join(root, "after")
	require.NoError(t, os.WriteFile(after, []byte("original"), 0600))
	m := &model{}
	m.recordUndo("move", []pathMove{{from: before, to: after}})
	require.NoError(t, os.WriteFile(after, []byte("edited"), 0600))
	require.NoError(t, m.undoLastFileOperation())
	data, err := os.ReadFile(before)
	require.NoError(t, err)
	require.Equal(t, "edited", string(data))
}

func TestUndoCannotCaptureAMissingItemAndBlessALaterReplacement(t *testing.T) {
	root := t.TempDir()
	before, after := filepath.Join(root, "before"), filepath.Join(root, "after")
	m := &model{}
	m.recordUndo("move", []pathMove{{from: before, to: after}})
	require.NoError(t, os.WriteFile(after, []byte("replacement"), 0600))
	require.ErrorContains(t, m.undoLastFileOperation(), "capture")
	require.NoFileExists(t, before)
	require.FileExists(t, after)
}
