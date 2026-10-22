package internal

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/processbar"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestRenameCanBeUndone(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "before.txt")
	newPath := filepath.Join(dir, "after.txt")
	utils.SetupFilesWithData(t, []byte("original"), oldPath)

	m := defaultTestModel(dir)
	setFilePanelSelectedItemByName(t, m.getFocusedFilePanel(), filepath.Base(oldPath))
	m.panelItemRename()
	m.getFocusedFilePanel().Rename.SetValue(filepath.Base(newPath))
	m.confirmRename()
	require.FileExists(t, newPath)

	require.NoError(t, m.undoLastFileOperation())
	assert.FileExists(t, oldPath)
	assert.NoFileExists(t, newPath)
}

func TestUndoNeverOverwritesARecreatedSource(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "before.txt")
	newPath := filepath.Join(dir, "after.txt")
	utils.SetupFilesWithData(t, []byte("original"), oldPath)

	m := defaultTestModel(dir)
	setFilePanelSelectedItemByName(t, m.getFocusedFilePanel(), filepath.Base(oldPath))
	m.panelItemRename()
	m.getFocusedFilePanel().Rename.SetValue(filepath.Base(newPath))
	m.confirmRename()
	utils.SetupFilesWithData(t, []byte("replacement"), oldPath)

	require.Error(t, m.undoLastFileOperation())
	oldContent, err := os.ReadFile(oldPath)
	require.NoError(t, err)
	newContent, err := os.ReadFile(newPath)
	require.NoError(t, err)
	assert.Equal(t, "replacement", string(oldContent))
	assert.Equal(t, "original", string(newContent))
}

func TestRenameNeverOverwritesExistingDestination(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	destination := filepath.Join(dir, "destination.txt")
	utils.SetupFilesWithData(t, []byte("source"), source)
	utils.SetupFilesWithData(t, []byte("destination"), destination)

	m := defaultTestModel(dir)
	setFilePanelSelectedItemByName(t, m.getFocusedFilePanel(), filepath.Base(source))
	m.panelItemRename()
	m.getFocusedFilePanel().Rename.SetValue(filepath.Base(destination))
	m.confirmRename()

	sourceContent, err := os.ReadFile(source)
	require.NoError(t, err)
	destinationContent, err := os.ReadFile(destination)
	require.NoError(t, err)
	assert.Equal(t, "source", string(sourceContent))
	assert.Equal(t, "destination", string(destinationContent))
	assert.Nil(t, m.lastUndo)
}

func TestCutMoveCanBeUndone(t *testing.T) {
	sourceDir := t.TempDir()
	destinationDir := t.TempDir()
	source := filepath.Join(sourceDir, "moved.txt")
	destination := filepath.Join(destinationDir, "moved.txt")
	utils.SetupFilesWithData(t, []byte("content"), source)

	m := defaultTestModel(destinationDir)
	processes := processbar.New()
	processes.ListenForChannelUpdates()
	m.clipboard.Reset(true)
	msg := m.executePasteOperation(
		&processes, destinationDir, []string{source}, true, m.clipboard.Generation(), 1,
	)
	pasteMsg, ok := msg.(PasteOperationMsg)
	require.True(t, ok)
	pasteMsg.ApplyToModel(m)
	require.FileExists(t, destination)

	require.NoError(t, m.undoLastFileOperation())
	assert.FileExists(t, source)
	assert.NoFileExists(t, destination)
}

func TestUndoHotkeyDefersWorkAndAppliesJournalUpdateFromMessage(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "before.txt")
	newPath := filepath.Join(dir, "after.txt")
	utils.SetupFiles(t, oldPath)

	m := defaultTestModel(dir)
	setFilePanelSelectedItemByName(t, m.getFocusedFilePanel(), filepath.Base(oldPath))
	m.panelItemRename()
	m.getFocusedFilePanel().Rename.SetValue(filepath.Base(newPath))
	m.confirmRename()
	require.NotEmpty(t, common.Hotkeys.UndoLastFileOperation)

	cmd := m.mainKey(common.Hotkeys.UndoLastFileOperation[0])
	require.NotNil(t, cmd)
	assert.NoFileExists(t, oldPath)
	assert.FileExists(t, newPath)
	require.NotNil(t, m.lastUndo)

	msg := cmd()
	undoMsg, ok := msg.(UndoOperationMsg)
	require.True(t, ok)

	assert.FileExists(t, oldPath)
	assert.NoFileExists(t, newPath)
	require.NotNil(t, m.lastUndo)

	undoMsg.ApplyToModel(m)
	assert.Nil(t, m.lastUndo)
}

func TestUndoMovePrimitiveNeverReplacesDestination(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	destination := filepath.Join(dir, "destination.txt")
	utils.SetupFilesWithData(t, []byte("source"), source)
	utils.SetupFilesWithData(t, []byte("destination"), destination)

	require.Error(t, moveElementNoReplace(source, destination))
	sourceContent, err := os.ReadFile(source)
	require.NoError(t, err)
	destinationContent, err := os.ReadFile(destination)
	require.NoError(t, err)
	assert.Equal(t, "source", string(sourceContent))
	assert.Equal(t, "destination", string(destinationContent))
}

func TestStalePasteCompletionDoesNotClearNewClipboard(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	m.clipboard.Reset(false)
	copyGeneration := m.clipboard.Generation()
	oldCopyCompletion := newPasteOperationMsgWithContext(
		processbar.Successful, 1, false, copyGeneration, nil,
	)

	newItem := filepath.Join(t.TempDir(), "new-cut.txt")
	utils.SetupFiles(t, newItem)
	m.clipboard.Reset(true)
	m.clipboard.Add(newItem)
	oldCopyCompletion.ApplyToModel(m)

	assert.True(t, m.clipboard.IsCut())
	assert.Equal(t, []string{newItem}, m.clipboard.GetItems())
}

func TestPartialMoveRemainsUndoableAfterFailure(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original.txt")
	moved := filepath.Join(dir, "moved.txt")
	utils.SetupFilesWithData(t, []byte("content"), moved)
	m := defaultTestModel(dir)
	m.clipboard.Reset(true)
	generation := m.clipboard.Generation()

	msg := newPasteOperationMsgWithContext(
		processbar.Failed,
		1,
		true,
		generation,
		[]pathMove{{from: original, to: moved}},
	)
	msg.ApplyToModel(m)

	require.NoError(t, m.undoLastFileOperation())
	assert.FileExists(t, original)
	assert.NoFileExists(t, moved)
}

func TestAsyncUndoKeepsOnlyUnfinishedMovesForRetry(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original.txt")
	firstMoved := filepath.Join(dir, "first.txt")
	secondMoved := filepath.Join(dir, "second.txt")
	utils.SetupFilesWithData(t, []byte("first"), firstMoved)
	utils.SetupFilesWithData(t, []byte("second"), secondMoved)

	m := defaultTestModel(dir)
	m.recordUndo("move", []pathMove{
		{from: original, to: firstMoved},
		{from: original, to: secondMoved},
	})
	cmd := m.mainKey(common.Hotkeys.UndoLastFileOperation[0])
	require.NotNil(t, cmd)

	msg, ok := cmd().(UndoOperationMsg)
	require.True(t, ok)
	require.Error(t, msg.err)
	require.Len(t, m.lastUndo.moves, 2)

	msg.ApplyToModel(m)
	require.Len(t, m.lastUndo.moves, 1)
	assert.Equal(t, firstMoved, m.lastUndo.moves[0].to)
	assert.FileExists(t, original)
	assert.FileExists(t, firstMoved)
	assert.NoFileExists(t, secondMoved)

	require.NoError(t, os.Remove(original))
	retryMsg, ok := m.mainKey(common.Hotkeys.UndoLastFileOperation[0])().(UndoOperationMsg)
	require.True(t, ok)
	require.NoError(t, retryMsg.err)
	retryMsg.ApplyToModel(m)
	assert.Nil(t, m.lastUndo)
	assert.FileExists(t, original)
	assert.NoFileExists(t, firstMoved)
}

func TestCaseOnlyRenameCanBeUndoneOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("case-insensitive path identity is Windows-specific")
	}
	dir := t.TempDir()
	original := filepath.Join(dir, "name.txt")
	renamed := filepath.Join(dir, "NAME.txt")
	utils.SetupFiles(t, original)
	m := defaultTestModel(dir)
	setFilePanelSelectedItemByName(t, m.getFocusedFilePanel(), filepath.Base(original))
	m.panelItemRename()
	m.getFocusedFilePanel().Rename.SetValue(filepath.Base(renamed))
	m.confirmRename()
	require.FileExists(t, renamed)

	require.NoError(t, m.undoLastFileOperation())
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, filepath.Base(original), entries[0].Name())
}
