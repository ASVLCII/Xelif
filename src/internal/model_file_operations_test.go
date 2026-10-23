package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/pkg/utils"

	variable "github.com/yorukot/superfile/src/config"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/notify"
)

// TODO : Add test for model initialized with multiple directories
// TODO : Add test for clipboard different variations, cut paste
// TODO : Add test for tea resizing
// TODO : Add test for quitting

func TestCopy(t *testing.T) {
	curTestDir := filepath.Join(testDir, "TestCopy")
	dir1 := filepath.Join(curTestDir, "dir1")
	dir2 := filepath.Join(curTestDir, "dir2")
	file1 := filepath.Join(dir1, "file1.txt")
	t.Run("Basic Copy", func(t *testing.T) {
		utils.SetupDirectories(t, curTestDir, dir1, dir2)
		utils.SetupFilesWithData(t, []byte("copied content"), file1)
		t.Cleanup(func() {
			os.RemoveAll(curTestDir)
		})

		p := NewTestTeaProgWithEventLoop(t, defaultTestModel(dir1))

		p.WithModel(func(m *model) {
			require.Equal(t, "file1.txt", m.getFocusedFilePanel().GetFocusedItem().Name)
		})
		p.SendKey(common.Hotkeys.CopyItems[0])
		p.WithModel(func(m *model) {
			assert.False(t, m.clipboard.IsCut())
			assert.Equal(t, file1, m.clipboard.GetFirstItem())
			require.NoError(t, m.updateCurrentFilePanelDir("../dir2"))
		})
		p.SendKey(common.Hotkeys.PasteItems[0])

		require.Eventually(t, func() bool {
			data, err := os.ReadFile(filepath.Join(dir2, "file1.txt"))
			return err == nil && string(data) == "copied content"
		}, DefaultTestTimeout, DefaultTestTick)

		p.WithModel(func(m *model) {
			assert.False(t, m.clipboard.IsCut())
			assert.Equal(t, file1, m.clipboard.GetFirstItem())
		})

		p.SendKey(common.Hotkeys.PasteItems[0])
		require.Eventually(t, func() bool {
			data, err := os.ReadFile(filepath.Join(dir2, "file1(1).txt"))
			return err == nil && string(data) == "copied content"
		}, DefaultTestTimeout, DefaultTestTick)
		for _, name := range []string{"file1.txt", "file1(1).txt"} {
			data, err := os.ReadFile(filepath.Join(dir2, name))
			require.NoError(t, err)
			assert.Equal(t, "copied content", string(data))
		}
		//TODO: Also verify if there are only 2 items in process bar
	})
}

func TestFileCreation(t *testing.T) {
	// TODO Also add directory creation test to this
	curTestDir := filepath.Join(testDir, "TestNaming")
	testParentDir := filepath.Join(curTestDir, "parentDir")
	testChildDir := filepath.Join(testParentDir, "childDir")

	utils.SetupDirectories(t, curTestDir, testParentDir, testChildDir)

	t.Cleanup(func() {
		os.RemoveAll(curTestDir)
	})

	testdata := []struct {
		name          string
		fileName      string
		expectedError bool
	}{
		{"valid name", "file.txt", false},
		{"invalid single dot", ".", true},
		{"invalid double dot", "..", true},
		{"invalid trailing slash-dot", fmt.Sprintf("test%c.", filepath.Separator), true},
		{"invalid trailing slash-dot-dot", fmt.Sprintf("test%c..", filepath.Separator), true},
		{"valid name with trailing .", "abc.", false},
	}

	for _, tt := range testdata {
		t.Run(tt.name, func(t *testing.T) {
			m := defaultTestModel(testChildDir)
			p := NewTestTeaProgWithEventLoop(t, m)
			p.SendKey(common.Hotkeys.FilePanelItemCreate[0])

			require.Eventually(t, func() bool {
				return p.Check(func(m *model) bool { return m.typingModal.open })
			}, DefaultTestTimeout, DefaultTestTick, "Typing modal never opened")

			p.SendKey(tt.fileName)
			p.SendKey(common.Hotkeys.ConfirmTyping[0])

			require.Eventually(t, func() bool {
				return p.Check(func(m *model) bool { return !m.typingModal.open })
			}, DefaultTestTimeout, DefaultTestTick, "Typing modal never closed")

			if tt.expectedError {
				require.Eventually(t, func() bool {
					return p.Check(func(m *model) bool { return m.spfError.IsOpen() })
				}, DefaultTestTimeout, DefaultTestTick, "SPF error modal never opened for input: %q", tt.fileName)

				p.SendKey(common.Hotkeys.Quit[0])
				require.Eventually(t, func() bool {
					return p.Check(func(m *model) bool { return !m.spfError.IsOpen() })
				}, DefaultTestTimeout, DefaultTestTick, "SPF error modal never closed")
				return
			}

			targetFile := filepath.Join(testChildDir, tt.fileName)
			assert.Eventually(t, func() bool {
				_, err := os.Lstat(targetFile)
				return err == nil
			}, DefaultTestTimeout, DefaultTestTick, "Target file did not get created : %q", targetFile)
			p.WithModel(func(m *model) {
				assert.False(t, m.spfError.IsOpen(), "Unexpected SPF error for input: %q", tt.fileName)
			})
		})
	}

	// This is to verify that even if m.typingModal.location is changed while the command is being executed
	// the create still works for older location set in typingModal
	t.Run("create request snapshots destination", func(t *testing.T) {
		originalLocation := t.TempDir()
		changedLocation := t.TempDir()
		m := defaultTestModel(originalLocation)
		m.panelCreateNewFile()
		m.typingModal.textInput.SetValue("captured.txt")

		cmd := m.getCreateCmd()
		require.NotNil(t, cmd)
		assert.False(t, m.typingModal.open)

		m.typingModal.location = changedLocation
		msg := cmd()
		require.IsType(t, CreateOperationMsg{}, msg)

		assert.FileExists(t, filepath.Join(originalLocation, "captured.txt"))
		assert.NoFileExists(t, filepath.Join(changedLocation, "captured.txt"))
	})
}

func TestFileRename(t *testing.T) {
	curTestDir := t.TempDir()
	file1 := filepath.Join(curTestDir, "file1.txt")
	file2 := filepath.Join(curTestDir, "file2.txt")
	file3 := filepath.Join(curTestDir, "file3.txt")

	utils.SetupFilesWithData(t, []byte("f1"), file1)
	utils.SetupFilesWithData(t, []byte("f2"), file2)
	utils.SetupFilesWithData(t, []byte("f3"), file3)

	file1New := filepath.Join(curTestDir, "file1_new.txt")

	t.Run("Basic rename", func(t *testing.T) {
		m := defaultTestModel(curTestDir)
		setFilePanelSelectedItemByLocation(t, m.getFocusedFilePanel(), file1)
		p := NewTestTeaProgWithEventLoop(t, m)

		p.SendKey(common.Hotkeys.FilePanelItemRename[0])
		p.SendKey("_new")
		p.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

		assert.Eventually(t, func() bool {
			_, err1 := os.Stat(file1)
			_, err1New := os.Stat(file1New)
			return err1New == nil && os.IsNotExist(err1)
		}, DefaultTestTimeout, DefaultTestTick, "File never got renamed")
	})

	t.Run("Rename never replaces an existing item", func(t *testing.T) {
		m := defaultTestModel(curTestDir)
		setFilePanelSelectedItemByLocation(t, m.getFocusedFilePanel(), file3)
		p := NewTestTeaProgWithEventLoop(t, m)

		p.SendKey(common.Hotkeys.FilePanelItemRename[0])
		p.Send(tea.KeyPressMsg{Code: tea.KeyBackspace})
		p.SendKey("2")
		p.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

		require.Eventually(t, func() bool {
			return p.Check(func(m *model) bool { return m.notifyModel.IsOpen() })
		}, DefaultTestTimeout, DefaultTestTick)
		p.WithModel(func(m *model) {
			assert.Equal(t, "Could not rename item", m.notifyModel.GetTitle())
		})
		assert.FileExists(t, file2)
		assert.FileExists(t, file3)
		file2Data, err := os.ReadFile(file2)
		require.NoError(t, err)
		file3Data, err := os.ReadFile(file3)
		require.NoError(t, err)
		assert.Equal(t, "f2", string(file2Data))
		assert.Equal(t, "f3", string(file3Data))
	})
}

func isTrashed(fileAbsPath string) bool {
	fileName := filepath.Base(fileAbsPath)
	switch runtime.GOOS {
	case utils.OsDarwin:
		_, err := os.Stat(filepath.Join(variable.DarwinTrashDirectory, fileName))
		return err == nil
	case utils.OsLinux:
		entries, err := os.ReadDir(variable.LinuxTrashDirectoryInfo)
		if err != nil {
			return false
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			data, err := os.ReadFile(filepath.Join(variable.LinuxTrashDirectoryInfo, entry.Name()))
			if err != nil {
				continue
			}

			for _, line := range strings.Split(string(data), "\n") {
				if line == "Path="+fileAbsPath {
					return true
				}
			}
		}
		return false
	default:
		return false
	}
}

func TestFileDelete(t *testing.T) {
	if runtime.GOOS == utils.OsWindows {
		t.Skip("Skipping for windows")
	}
	curTestDir := t.TempDir()
	file1 := filepath.Join(curTestDir, "file1.txt")
	file2 := filepath.Join(curTestDir, "file2.txt")

	utils.SetupFilesWithData(t, []byte("f1"), file1)
	utils.SetupFilesWithData(t, []byte("f2"), file2)

	testdata := []struct {
		name            string
		filePath        string
		permanentDelete bool
	}{
		{
			name:            "Move to trash",
			filePath:        file1,
			permanentDelete: false,
		},
		{
			name:            "Permanently delete",
			filePath:        file2,
			permanentDelete: true,
		},
	}

	for _, tt := range testdata {
		t.Run(tt.name, func(t *testing.T) {
			m := defaultTestModel(curTestDir)
			m.hasTrash = common.InitTrash()
			setFilePanelSelectedItemByLocation(t, m.getFocusedFilePanel(), tt.filePath)
			p := NewTestTeaProgWithEventLoop(t, m)
			if tt.permanentDelete {
				p.SendKey(common.Hotkeys.PermanentlyDeleteItems[0])
			} else {
				p.SendKey(common.Hotkeys.DeleteItems[0])
			}
			assert.Eventually(t, func() bool {
				return p.Check(func(m *model) bool { return m.notifyModel.IsOpen() })
			}, DefaultTestTimeout,
				DefaultTestTick, "Notify model never opened")
			expectedTitle := common.TrashWarnTitle
			expectedAction := notify.DeleteAction
			if tt.permanentDelete {
				expectedTitle = common.PermanentDeleteWarnTitle
				expectedAction = notify.PermanentDeleteAction
			}
			p.WithModel(func(m *model) {
				assert.Equal(t, expectedTitle, m.notifyModel.GetTitle())
				assert.Equal(t, expectedAction, m.notifyModel.GetConfirmAction())
			})

			p.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

			assert.Eventually(t, func() bool {
				_, err := os.Stat(tt.filePath)
				return err != nil && os.IsNotExist(err)
			}, DefaultTestTimeout, DefaultTestTick, "File never removed from original location")

			// Window's trash is not flexible enough for the check.
			// Sorry windows
			if runtime.GOOS == utils.OsDarwin || runtime.GOOS == utils.OsLinux {
				assert.Equal(t, tt.permanentDelete, !isTrashed(tt.filePath),
					"Existence in trash status should be expected only of not permanently deleted")
			}
		})
	}
}
