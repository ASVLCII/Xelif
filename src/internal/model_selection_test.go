package internal

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestAdvancedSelectionHotkeys(t *testing.T) {
	dir := t.TempDir()
	directory := filepath.Join(dir, "folder")
	fileA := filepath.Join(dir, "a.txt")
	fileB := filepath.Join(dir, "b.txt")
	utils.SetupDirectories(t, directory)
	utils.SetupFiles(t, fileA, fileB)

	t.Run("range", func(t *testing.T) {
		m := defaultTestModel(dir)
		panel := m.getFocusedFilePanel()
		panel.ChangeFilePanelMode()
		require.NotEmpty(t, common.Hotkeys.FilePanelSelectRange)
		require.True(t, panel.FocusIndex(0))
		m.mainKey(common.Hotkeys.FilePanelSelectRange[0])
		require.True(t, panel.FocusIndex(panel.ElemCount()-1))

		m.mainKey(common.Hotkeys.FilePanelSelectRange[0])

		assert.Equal(t, uint(panel.ElemCount()), panel.SelectedCount())
	})

	t.Run("invert", func(t *testing.T) {
		m := defaultTestModel(dir)
		panel := m.getFocusedFilePanel()
		panel.ChangeFilePanelMode()
		panel.SetSelected(fileA)
		require.NotEmpty(t, common.Hotkeys.FilePanelInvertSelection)

		m.mainKey(common.Hotkeys.FilePanelInvertSelection[0])

		assert.False(t, panel.CheckSelected(fileA))
		assert.True(t, panel.CheckSelected(fileB))
		assert.True(t, panel.CheckSelected(directory))
	})

	t.Run("files and directories", func(t *testing.T) {
		m := defaultTestModel(dir)
		panel := m.getFocusedFilePanel()
		panel.ChangeFilePanelMode()
		require.NotEmpty(t, common.Hotkeys.FilePanelSelectFiles)
		require.NotEmpty(t, common.Hotkeys.FilePanelSelectDirectories)

		m.mainKey(common.Hotkeys.FilePanelSelectFiles[0])
		assert.True(t, panel.CheckSelected(fileA))
		assert.True(t, panel.CheckSelected(fileB))
		assert.False(t, panel.CheckSelected(directory))

		panel.ResetSelected()
		m.mainKey(common.Hotkeys.FilePanelSelectDirectories[0])
		assert.False(t, panel.CheckSelected(fileA))
		assert.False(t, panel.CheckSelected(fileB))
		assert.True(t, panel.CheckSelected(directory))
	})
}
