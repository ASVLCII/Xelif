package internal

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/ui/filepanel"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestModifierClickSelectsTheSameFilesAsKeyboardSelection(t *testing.T) {
	dir := t.TempDir()
	utils.SetupFiles(t, filepath.Join(dir, "a"), filepath.Join(dir, "b"), filepath.Join(dir, "c"))
	m := defaultTestModel(dir)
	panel := m.getFocusedFilePanel()
	x, y := fileEntryScreenPosition(m, 0)
	TeaUpdate(m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft, Mod: tea.ModCtrl})
	assert.True(t, panel.CheckSelected(filepath.Join(dir, "a")))
	assert.Equal(t, filepanel.SelectMode, panel.PanelMode)
	x, y = fileEntryScreenPosition(m, 2)
	TeaUpdate(m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft, Mod: tea.ModShift})
	assert.EqualValues(t, 3, panel.SelectedCount())
	assert.Equal(t, 2, panel.GetCursor())
	assert.Equal(t, dir, panel.Location)
}

func TestKeyboardOpensContextMenuWithoutMouse(t *testing.T) {
	dir := t.TempDir()
	utils.SetupFiles(t, filepath.Join(dir, "a"))
	m := defaultTestModel(dir)
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyF10, Mod: tea.ModShift})
	require.True(t, m.contextMenu.open)
	assert.Equal(t, filepath.Join(dir, "a"), m.contextMenu.targetPath)
}

func TestMouseCanOpenSearchAndCommandsAndCreateCloseTabs(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	panel := m.getFocusedFilePanel()
	panelX := m.sidebarModel.GetWidth() + 1
	TeaUpdate(m, clickAt(panelX, 2, tea.MouseLeft))
	require.True(t, m.globalSearchModal.IsOpen())
	m.globalSearchModal.Close()
	TeaUpdate(m, clickAt(panelX+10, 2, tea.MouseLeft))
	require.True(t, m.commandPalette.IsOpen())
	m.commandPalette.Close()
	for _, createTab := range []bool{true, false} {
		controlX := -1
		for x := range panel.GetContentWidth() {
			create, closeTab := panel.TabControlAtContentX(x)
			if (createTab && create) || (!createTab && closeTab) {
				controlX = x
				break
			}
		}
		require.NotEqual(t, -1, controlX)
		TeaUpdate(m, clickAt(panelX+controlX, 1, tea.MouseLeft))
		if createTab {
			assert.Equal(t, 2, panel.TabCount())
		} else {
			assert.Equal(t, 1, panel.TabCount())
		}
	}
}
