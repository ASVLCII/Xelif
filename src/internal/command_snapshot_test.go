package internal

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/trash"
	"github.com/yorukot/superfile/src/internal/ui/notify"
	"github.com/yorukot/superfile/src/internal/ui/processbar"
)

func TestCompressCommandKeepsSubmittedDirectory(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()
	const name = "command-snapshot-source.txt"
	const contents = "archive the originally selected file"
	require.NoError(t, os.WriteFile(filepath.Join(source, name), []byte(contents), 0o600))
	m := defaultTestModel(source)
	panel := m.getFocusedFilePanel()
	setFilePanelSelectedItemByName(t, panel, name)
	cmd := m.getCompressSelectedFilesCmd()
	require.NotNil(t, cmd)
	require.NoError(t, panel.UpdateCurrentFilePanelDir(destination))

	// This single-file operation fits in the process message buffer. No event
	// loop or background listener shares the model with the test.
	msg, ok := cmd().(CompressOperationMsg)
	require.True(t, ok)
	require.Equal(t, processbar.Successful, msg.state)
	archivePath := filepath.Join(source, "command-snapshot-source.zip")
	reader, err := zip.OpenReader(archivePath)
	require.NoError(t, err)
	defer reader.Close()
	require.Len(t, reader.File, 1)
	require.Equal(t, name, reader.File[0].Name)
	file, err := reader.File[0].Open()
	require.NoError(t, err)
	defer file.Close()
	archived, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, contents, string(archived))
	require.NoFileExists(t, filepath.Join(destination, "command-snapshot-source.zip"))
}

func TestDeleteWarningCommandKeepsSubmittedTrashCapability(t *testing.T) {
	source := t.TempDir()
	if !trash.Available(source) {
		t.Skip("trash backend unavailable")
	}
	require.NoError(t, os.WriteFile(filepath.Join(source, "selected.txt"), []byte("keep"), 0o600))
	m := defaultTestModel(source)
	m.hasTrash = false
	cmd := m.getDeleteTriggerCmd(false)
	require.NotNil(t, cmd)
	m.hasTrash = true
	msg, ok := cmd().(NotifyModalUpdateMsg)
	require.True(t, ok)
	msg.ApplyToModel(m)
	require.Equal(t, common.PermanentDeleteWarnTitle, m.notifyModel.GetTitle())
	require.Equal(t, notify.PermanentDeleteAction, m.notifyModel.GetConfirmAction())
}

func TestDeleteWarningCommandKeepsSubmittedDirectory(t *testing.T) {
	source := t.TempDir()
	if !trash.Available(source) || trash.Available("") {
		t.Skip("trash backend does not distinguish an unavailable empty location")
	}
	require.NoError(t, os.WriteFile(filepath.Join(source, "selected.txt"), []byte("keep"), 0o600))
	m := defaultTestModel(source)
	m.hasTrash = true
	cmd := m.getDeleteTriggerCmd(false)
	require.NotNil(t, cmd)
	// Poison later panel state so consulting it from the worker changes the
	// warning, without timing-dependent concurrent mutation.
	m.getFocusedFilePanel().Location = ""
	msg, ok := cmd().(NotifyModalUpdateMsg)
	require.True(t, ok)
	msg.ApplyToModel(m)
	require.Equal(t, common.TrashWarnTitle, m.notifyModel.GetTitle())
	require.Equal(t, notify.DeleteAction, m.notifyModel.GetConfirmAction())
}
