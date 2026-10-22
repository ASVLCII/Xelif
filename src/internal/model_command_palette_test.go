package internal

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	variable "github.com/yorukot/superfile/src/config"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/commandpalette"
)

func TestCommandPaletteHotkeyOpens(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	require.NotEmpty(t, common.Hotkeys.OpenCommandPalette)

	m.mainKey(common.Hotkeys.OpenCommandPalette[0])

	assert.True(t, m.commandPalette.IsOpen())
}

func TestCommandPaletteExecutesExistingToggleAction(t *testing.T) {
	originalToggleFile := variable.ToggleDotFile
	//nolint:reassign // Persist the toggle only inside this test's temporary directory.
	variable.ToggleDotFile = filepath.Join(t.TempDir(), "toggle-dot-files")
	t.Cleanup(func() {
		//nolint:reassign // Restore the toggle store path after this isolated test.
		variable.ToggleDotFile = originalToggleFile
	})
	m := defaultTestModel(t.TempDir())
	before := m.fileModel.DisplayDotFiles

	m.applyCommandPaletteAction(commandpalette.ExecuteAction{ID: commandIDToggleDotFiles})

	assert.Equal(t, !before, m.fileModel.DisplayDotFiles)
}

func TestCommandPaletteDisablesItemActionsWithoutSelection(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	require.True(t, m.getFocusedFilePanel().Empty())

	items := m.paletteItems()
	var rename commandpalette.Item
	for _, item := range items {
		if item.ID == commandIDRename {
			rename = item
			break
		}
	}

	assert.Equal(t, commandIDRename, rename.ID)
	assert.True(t, rename.Disabled)
	assert.NotEmpty(t, rename.DisabledReason)
}

func TestCommandPaletteEscapeClosesOnlyPalette(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	m.openCommandPalette()

	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	assert.False(t, m.commandPalette.IsOpen())
	assert.Equal(t, notQuitting, m.modelQuitState)
}

func TestCommandPaletteResultIsClickable(t *testing.T) {
	originalToggleFile := variable.ToggleDotFile
	//nolint:reassign // Persist the toggle only inside this test's temporary directory.
	variable.ToggleDotFile = filepath.Join(t.TempDir(), "toggle-dot-files")
	t.Cleanup(func() {
		//nolint:reassign // Restore the toggle store path after this isolated test.
		variable.ToggleDotFile = originalToggleFile
	})
	m := defaultTestModel(t.TempDir())
	m.commandPalette = commandpalette.New([]commandpalette.Item{{
		ID: commandIDToggleDotFiles, Label: "Toggle hidden files",
	}})
	m.setCommandPaletteSize()
	m.commandPalette.Open()
	before := m.fileModel.DisplayDotFiles
	overlayX := m.fullWidth/common.CenterDivisor - m.commandPalette.GetWidth()/common.CenterDivisor
	overlayY := m.fullHeight/common.CenterDivisor - m.commandPalette.GetMaxHeight()/common.CenterDivisor

	TeaUpdate(m, clickAt(overlayX+1, overlayY+3, tea.MouseLeft))

	assert.Equal(t, !before, m.fileModel.DisplayDotFiles)
	assert.False(t, m.commandPalette.IsOpen())
}
