package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	variable "github.com/yorukot/superfile/src/config"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func favouriteTestModel(t *testing.T, root string) *model {
	t.Helper()
	oldPath, oldConfig := variable.PinnedFile, common.Config
	t.Cleanup(func() {
		variable.PinnedFile = oldPath //nolint:reassign // Restore test-only storage override.
		common.SetConfig(oldConfig)
	})
	variable.PinnedFile = filepath.Join(t.TempDir(), "pinned.json") //nolint:reassign // Isolate user favourites.
	config := oldConfig
	config.SidebarWidth = 20
	config.SidebarSections = []string{utils.SidebarSectionPinned}
	common.SetConfig(config)
	return defaultTestModel(root)
}

func savedFavouritePaths(t *testing.T) []string {
	t.Helper()
	content, err := os.ReadFile(variable.PinnedFile)
	require.NoError(t, err)
	var entries []struct {
		Location string `json:"location"`
	}
	require.NoError(t, json.Unmarshal(content, &entries))
	paths := make([]string, len(entries))
	for i := range entries {
		paths[i] = entries[i].Location
	}
	return paths
}

func TestFavouritesContextAndSidebarKeyboardUseSameStorage(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "project")
	require.NoError(t, os.Mkdir(child, 0o700))
	m := favouriteTestModel(t, root)
	index := m.getFocusedFilePanel().FindElementIndexByLocation(child)
	require.GreaterOrEqual(t, index, 0)
	x, y := fileEntryScreenPosition(m, index)
	TeaUpdate(m, clickAt(x, y, tea.MouseRight))
	require.True(t, m.contextMenu.open)
	TeaUpdate(m, clickAt(m.contextMenu.x+2, m.contextMenu.y+1+contextFavourite, tea.MouseLeft))
	require.Equal(t, []string{child}, savedFavouritePaths(t))
	m.sidebarModel.UpdateDirectories()
	TeaUpdate(m, clickAt(2, 4, tea.MouseRight))
	require.True(t, m.contextMenu.sidebarTarget)
	require.Len(t, m.contextMenuLabels(), 1)
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Empty(t, savedFavouritePaths(t))
	m.focusPanel = nonePanelFocus
	TeaUpdate(m, utils.TeaRuneKeyMsg(common.Hotkeys.PinnedDirectory[0]))
	require.Equal(t, []string{root}, savedFavouritePaths(t))
	m.sidebarModel.UpdateDirectories()
	_, hit := m.sidebarModel.FocusDirectoryAtY(4, true)
	require.True(t, hit)
	m.focusPanel = sidebarFocus
	m.openKeyboardContextMenu()
	require.True(t, m.contextMenu.sidebarTarget)
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	TeaUpdate(m, utils.TeaRuneKeyMsg(common.Hotkeys.PinnedDirectory[0]))
	assert.Empty(t, savedFavouritePaths(t))
}

func TestFavouritesCorruptStorageErrorIsVisibleAndNonDestructive(t *testing.T) {
	m := favouriteTestModel(t, t.TempDir())
	before := []byte("{broken favourites")
	require.NoError(t, os.WriteFile(variable.PinnedFile, before, 0o600))
	TeaUpdate(m, utils.TeaRuneKeyMsg(common.Hotkeys.PinnedDirectory[0]))
	assert.True(t, m.notifyModel.IsOpen())
	after, err := os.ReadFile(variable.PinnedFile)
	require.NoError(t, err)
	assert.Equal(t, before, after)
}
