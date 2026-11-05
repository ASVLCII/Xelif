package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/processbar"
	"github.com/yorukot/superfile/src/internal/ui/spferror"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestContextCreateFolderUsesExistingOperation(t *testing.T) {
	location := t.TempDir()
	otherLocation := t.TempDir()
	m := defaultTestModel(location)
	m.panelCreateNewFile()
	m.typingModal.directory = true
	m.typingModal.textInput.SetValue("New folder")
	assert.NoDirExists(t, filepath.Join(location, "New folder"))

	cmd := m.getCreateCmd()
	require.NotNil(t, cmd)
	assert.False(t, m.typingModal.open)
	assert.NoDirExists(t, filepath.Join(location, "New folder"))

	// A submitted operation must not observe later dialog edits or mode changes.
	m.typingModal.location = otherLocation
	m.typingModal.directory = false
	m.typingModal.textInput.SetValue("later.txt")
	msg, ok := cmd().(CreateOperationMsg)
	require.True(t, ok)
	assert.Equal(t, processbar.Successful, msg.state)
	assert.DirExists(t, filepath.Join(location, "New folder"))
	assert.NoDirExists(t, filepath.Join(otherLocation, "New folder"))
	assert.NoFileExists(t, filepath.Join(otherLocation, "later.txt"))
}

func TestContextCreateFolderCancellationWritesNothing(t *testing.T) {
	location := t.TempDir()
	m := defaultTestModel(location)
	m.panelCreateNewFile()
	m.typingModal.directory = true
	m.typingModal.textInput.SetValue("cancelled")

	TeaUpdate(m, utils.TeaRuneKeyMsg(common.Hotkeys.CancelTyping[0]))
	assert.False(t, m.typingModal.open)
	assert.False(t, m.typingModal.textInput.Focused())
	assert.Nil(t, m.getCreateCmd())
	entries, err := os.ReadDir(location)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestContextCreateFolderEmptyNameWritesNothing(t *testing.T) {
	location := t.TempDir()
	m := defaultTestModel(location)
	m.panelCreateNewFile()
	m.typingModal.directory = true

	assert.Nil(t, m.getCreateCmd())
	assert.True(t, m.typingModal.open)
	entries, err := os.ReadDir(location)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestContextCreateFolderDoesNotChangeOrdinaryCreate(t *testing.T) {
	location := t.TempDir()
	m := defaultTestModel(location)
	m.panelCreateNewFile()
	m.typingModal.directory = true
	m.typingModal.textInput.Placeholder = "Folder name"
	m.cancelTypingModal()

	m.mainKey(common.Hotkeys.FilePanelItemCreate[0])
	require.True(t, m.typingModal.open)
	assert.False(t, m.typingModal.directory)
	assert.Equal(t, common.GenerateNewFileTextInput().Placeholder, m.typingModal.textInput.Placeholder)
	m.typingModal.textInput.SetValue("ordinary.txt")
	cmd := m.getCreateCmd()
	require.NotNil(t, cmd)
	msg, ok := cmd().(CreateOperationMsg)
	require.True(t, ok)
	assert.Equal(t, processbar.Successful, msg.state)
	assert.FileExists(t, filepath.Join(location, "ordinary.txt"))

	// The generic create dialog still accepts its existing directory shortcut.
	m.mainKey(common.Hotkeys.FilePanelItemCreate[0])
	m.typingModal.textInput.SetValue("shortcut" + string(filepath.Separator))
	cmd = m.getCreateCmd()
	require.NotNil(t, cmd)
	msg, ok = cmd().(CreateOperationMsg)
	require.True(t, ok)
	assert.Equal(t, processbar.Successful, msg.state)
	assert.DirExists(t, filepath.Join(location, "shortcut"))
}

func TestContextCreateFolderPreservesNameValidation(t *testing.T) {
	for _, name := range []string{".", "..", "nested" + string(filepath.Separator) + "."} {
		t.Run(name, func(t *testing.T) {
			location := t.TempDir()
			m := defaultTestModel(location)
			m.panelCreateNewFile()
			m.typingModal.directory = true
			m.typingModal.textInput.SetValue(name)
			cmd := m.getCreateCmd()
			require.NotNil(t, cmd)
			msg := cmd()
			require.IsType(t, SpfErrorModalUpdateMsg{}, msg)
			TeaUpdate(m, msg)
			assert.True(t, m.spfError.IsOpen())
			abort := m.spfErrorModelOpenKey(spferror.KeyAbort()[0])
			require.NotNil(t, abort)
			require.IsType(t, CreateOperationMsg{}, abort())
			entries, err := os.ReadDir(location)
			require.NoError(t, err)
			assert.Empty(t, entries)
		})
	}
}

func TestContextPasteIntoSnapshotsDestination(t *testing.T) {
	location := t.TempDir()
	destination := t.TempDir()
	otherLocation := t.TempDir()
	source := filepath.Join(location, "source.txt")
	require.NoError(t, os.WriteFile(source, []byte("copied content"), utils.UserFilePerm))
	m := defaultTestModel(location)
	m.clipboard.Add(source)

	cmd := m.getPasteIntoCmd(destination)
	require.NotNil(t, cmd)
	assert.Equal(t, location, m.getFocusedFilePanel().Location)
	assert.NoFileExists(t, filepath.Join(destination, "source.txt"))
	m.getFocusedFilePanel().Location = otherLocation
	m.clipboard.Reset(false)
	msg, ok := cmd().(PasteOperationMsg)
	require.True(t, ok)
	assert.Equal(t, processbar.Successful, msg.state)
	data, err := os.ReadFile(filepath.Join(destination, "source.txt"))
	require.NoError(t, err)
	assert.Equal(t, "copied content", string(data))
	assert.NoFileExists(t, filepath.Join(otherLocation, "source.txt"))
}

func TestContextPasteIntoPreservesValidation(t *testing.T) {
	location := t.TempDir()
	source := filepath.Join(location, "source")
	destination := filepath.Join(source, "nested")
	require.NoError(t, os.MkdirAll(destination, utils.UserDirPerm))
	m := defaultTestModel(location)
	m.clipboard.Add(source)
	cmd := m.getPasteIntoCmd(destination)
	require.NotNil(t, cmd)
	msg, ok := cmd().(NotifyModalUpdateMsg)
	require.True(t, ok)
	assert.Equal(t, "Invalid paste location", msg.m.GetTitle())
	assert.NoDirExists(t, filepath.Join(destination, "source"))
	assert.Equal(t, location, m.getFocusedFilePanel().Location)
}
