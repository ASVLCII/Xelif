package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	variable "github.com/yorukot/superfile/src/config"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestChooserWritesAllSelectedPaths(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "folder")
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")
	utils.SetupDirectories(t, dir, folder)
	utils.SetupFiles(t, a, b)
	old := variable.ChooserFile
	t.Cleanup(func() {
		//nolint:reassign // Restore the chooser output path after this isolated test.
		variable.ChooserFile = old
	})
	//nolint:reassign // Keep chooser output inside the test's temporary directory.
	variable.ChooserFile = filepath.Join(t.TempDir(), "chosen")

	for _, editor := range []bool{false, true} {
		m := defaultTestModel(dir)
		panel := m.getFocusedFilePanel()
		panel.ChangeFilePanelMode()
		panel.SetSelected(b)
		panel.SetSelected(a)
		setFilePanelSelectedItemByLocation(t, panel, folder)
		if editor {
			_ = m.openFileWithEditor()
		} else {
			m.enterPanel()
		}
		data, err := os.ReadFile(variable.ChooserFile)
		require.NoError(t, err)
		require.Equal(t, strings.Join([]string{a, b}, "\n"), string(data))
		require.Equal(t, quitInitiated, m.modelQuitState)
		require.Equal(t, dir, panel.Location)
	}
}

func TestChooserFocusedPathAndWriteFailure(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	utils.SetupFiles(t, file)
	m := defaultTestModel(dir)
	setFilePanelSelectedItemByLocation(t, m.getFocusedFilePanel(), file)
	require.Equal(t, []string{file}, m.chooserPaths())
	old := variable.ChooserFile
	t.Cleanup(func() {
		//nolint:reassign // Restore the chooser output path after this isolated test.
		variable.ChooserFile = old
	})
	//nolint:reassign // Exercise a write failure without touching the user's chooser output.
	variable.ChooserFile = filepath.Join(dir, "missing", "chosen")
	require.Error(t, m.chooserFileWriteAndQuit(m.chooserPaths()...))
	require.NotEqual(t, quitInitiated, m.modelQuitState)

	//nolint:reassign // Test successful chooser output at a writable temporary path.
	variable.ChooserFile = filepath.Join(dir, "chosen")
	require.NoError(t, m.chooserFileWriteAndQuit(m.chooserPaths()...))
	data, err := os.ReadFile(variable.ChooserFile)
	require.NoError(t, err)
	require.Equal(t, file, string(data))
}
