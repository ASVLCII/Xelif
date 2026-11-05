package internal

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/trash"
	"github.com/yorukot/superfile/src/internal/ui/notify"
	"github.com/yorukot/superfile/src/internal/ui/processbar"
)

func applyDeleteSnapshotWarning(t *testing.T, m *model, cmd tea.Cmd) NotifyModalUpdateMsg {
	t.Helper()
	require.NotNil(t, cmd)
	msg, ok := cmd().(NotifyModalUpdateMsg)
	require.True(t, ok)
	msg.ApplyToModel(m)
	return msg
}

func confirmDeleteSnapshot(t *testing.T, m *model) tea.Cmd {
	t.Helper()
	require.True(t, m.notifyModel.IsOpen())
	action := m.notifyModel.GetConfirmAction()
	m.notifyModel.Close()
	return m.handleNotifyModelConfirm(action)
}

func runDeleteSnapshot(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	require.NotNil(t, cmd)
	msg, ok := cmd().(DeleteOperationMsg)
	require.True(t, ok)
	require.Equal(t, processbar.Successful, msg.state)
}

func TestDeleteConfirmationSnapshotSurvivesExternalRenameAndFocusChange(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "a.txt")
	moved := filepath.Join(dir, "a-moved.txt")
	other := filepath.Join(dir, "b.txt")
	require.NoError(t, os.WriteFile(original, []byte("a"), 0o600))
	require.NoError(t, os.WriteFile(other, []byte("b"), 0o600))
	m := defaultTestModel(dir)
	defer m.processBarModel.Shutdown()
	m.hasTrash = false
	panel := m.getFocusedFilePanel()
	setFilePanelSelectedItemByLocation(t, panel, original)

	applyDeleteSnapshotWarning(t, m, m.getDeleteTriggerCmd(false))
	require.Equal(t, common.PermanentDeleteWarnTitle, m.notifyModel.GetTitle())
	require.Equal(t, common.PermanentDeleteWarnContent, m.notifyModel.GetContent())
	require.NoError(t, os.Rename(original, moved))
	panel.UpdateElementsIfNeeded(true, false)
	setFilePanelSelectedItemByLocation(t, panel, other)

	runDeleteSnapshot(t, confirmDeleteSnapshot(t, m))
	require.FileExists(t, other)
	require.FileExists(t, moved)
	require.NoFileExists(t, original)
}

func TestDeleteConfirmationSnapshotKeepsMultiplePathsAndFolder(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()
	a := filepath.Join(source, "a.txt")
	b := filepath.Join(source, "b.txt")
	c := filepath.Join(source, "c.txt")
	destinationFile := filepath.Join(destination, "a.txt")
	for _, path := range []string{a, b, c, destinationFile} {
		require.NoError(t, os.WriteFile(path, []byte(filepath.Base(path)), 0o600))
	}
	m := defaultTestModel(source)
	defer m.processBarModel.Shutdown()
	m.hasTrash = false
	panel := m.getFocusedFilePanel()
	setupPanelModeAndSelection(t, m, true, "", []string{a, c})

	warning := m.getDeleteTriggerCmd(false)
	require.NotNil(t, warning)
	require.Equal(t, source, m.pendingDelete.folder)
	require.ElementsMatch(t, []string{a, c}, m.pendingDelete.paths)
	panel.ResetSelected()
	panel.SetSelected(b)
	require.NoError(t, panel.UpdateCurrentFilePanelDir(destination))
	applyDeleteSnapshotWarning(t, m, warning)

	runDeleteSnapshot(t, confirmDeleteSnapshot(t, m))
	require.NoFileExists(t, a)
	require.NoFileExists(t, c)
	require.FileExists(t, b)
	require.FileExists(t, destinationFile)
}

func TestDeleteConfirmationSnapshotCancelAndUnconfirmedAreSafe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	require.NoError(t, os.WriteFile(path, []byte("a"), 0o600))
	m := defaultTestModel(dir)
	defer m.processBarModel.Shutdown()
	m.hasTrash = false

	warning := applyDeleteSnapshotWarning(t, m, m.getDeleteTriggerCmd(false))
	require.NotNil(t, m.pendingDelete)
	m.notifyModel.Close()
	require.Nil(t, m.handleNotifyModelCancel(notify.PermanentDeleteAction))
	require.Nil(t, m.pendingDelete)
	require.Nil(t, m.handleNotifyModelConfirm(notify.PermanentDeleteAction))
	warning.ApplyToModel(m)
	require.False(t, m.notifyModel.IsOpen())

	// A confirmation cannot borrow the current focus before its own warning.
	lateWarning := m.getDeleteTriggerCmd(true)
	require.Nil(t, m.handleNotifyModelConfirm(notify.PermanentDeleteAction))
	applyDeleteSnapshotWarning(t, m, lateWarning)
	require.False(t, m.notifyModel.IsOpen())
	require.FileExists(t, path)
}

func TestDeleteConfirmationSnapshotIgnoresOutOfOrderWarnings(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	require.NoError(t, os.WriteFile(a, []byte("a"), 0o600))
	require.NoError(t, os.WriteFile(b, []byte("b"), 0o600))
	m := defaultTestModel(dir)
	defer m.processBarModel.Shutdown()
	m.hasTrash = false
	panel := m.getFocusedFilePanel()
	setFilePanelSelectedItemByLocation(t, panel, a)
	first := m.getDeleteTriggerCmd(false)
	setFilePanelSelectedItemByLocation(t, panel, b)
	second := m.getDeleteTriggerCmd(false)

	oldWarning := applyDeleteSnapshotWarning(t, m, first)
	require.False(t, m.notifyModel.IsOpen())
	applyDeleteSnapshotWarning(t, m, second)
	require.True(t, m.notifyModel.IsOpen())
	oldWarning.ApplyToModel(m)
	require.Equal(t, []string{b}, m.pendingDelete.paths)
	runDeleteSnapshot(t, confirmDeleteSnapshot(t, m))
	oldWarning.ApplyToModel(m)
	require.False(t, m.notifyModel.IsOpen())
	require.FileExists(t, a)
	require.NoFileExists(t, b)
}

func TestDeleteConfirmationSnapshotOldDialogCannotConfirmNewRequest(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	require.NoError(t, os.WriteFile(a, []byte("a"), 0o600))
	require.NoError(t, os.WriteFile(b, []byte("b"), 0o600))
	m := defaultTestModel(dir)
	defer m.processBarModel.Shutdown()
	m.hasTrash = false
	panel := m.getFocusedFilePanel()
	setFilePanelSelectedItemByLocation(t, panel, a)
	applyDeleteSnapshotWarning(t, m, m.getDeleteTriggerCmd(false))
	setFilePanelSelectedItemByLocation(t, panel, b)
	newWarning := m.getDeleteTriggerCmd(false)

	// The old dialog is still visible, but the new request has not displayed
	// its warning. Confirming it must not operate on either selection.
	require.Nil(t, confirmDeleteSnapshot(t, m))
	applyDeleteSnapshotWarning(t, m, newWarning)
	require.False(t, m.notifyModel.IsOpen())
	require.Nil(t, m.pendingDelete)
	require.FileExists(t, a)
	require.FileExists(t, b)
}

func TestDeleteConfirmationSnapshotTrashChoiceDoesNotDowngrade(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	require.NoError(t, os.WriteFile(path, []byte("a"), 0o600))
	if !trash.Available(dir) {
		t.Skip("trash backend unavailable")
	}
	m := defaultTestModel(dir)
	defer m.processBarModel.Shutdown()
	m.hasTrash = true
	applyDeleteSnapshotWarning(t, m, m.getDeleteTriggerCmd(false))
	require.Equal(t, notify.DeleteAction, m.notifyModel.GetConfirmAction())
	// Simulate the state changing after the user sees the trash warning.
	m.hasTrash = false
	m.getFocusedFilePanel().Location = ""
	items, useTrash := m.takeDeleteRequest(false)
	require.Equal(t, []string{path}, items)
	require.True(t, useTrash)
	require.FileExists(t, path)
}

func TestDeleteConfirmationSnapshotOtherNotificationInvalidatesPending(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	require.NoError(t, os.WriteFile(path, []byte("a"), 0o600))
	m := defaultTestModel(dir)
	defer m.processBarModel.Shutdown()
	warning := m.getDeleteTriggerCmd(true)
	NewNotifyModalMsg(notify.New(true, "Other", "Other", notify.NoAction), m.nextIoReqCnt()).ApplyToModel(m)
	require.Nil(t, m.pendingDelete)
	applyDeleteSnapshotWarning(t, m, warning)
	require.Equal(t, "Other", m.notifyModel.GetTitle())
	require.FileExists(t, path)
}
