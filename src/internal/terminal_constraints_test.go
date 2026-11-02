package internal

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestResizeMessageBlocksHiddenFileActions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "keep.txt")
	require.NoError(t, os.WriteFile(path, []byte("untouched"), 0o600))
	m := dailyDriverTestModel(root)
	TeaUpdate(m, tea.WindowSizeMsg{Width: 59, Height: 24})
	for _, key := range []string{"f2", "shift+delete", "delete", "ctrl+v", "enter"} {
		TeaUpdate(m, utils.TeaRuneKeyMsg(key))
		require.False(t, m.fileModel.Renaming)
		require.False(t, m.notifyModel.IsOpen())
		require.Equal(t, root, m.getFocusedFilePanel().Location)
	}
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "untouched", string(data))
	TeaUpdate(m, tea.WindowSizeMsg{Width: 120, Height: 36})
	require.Equal(t, path, m.getFocusedFilePanel().GetFocusedItem().Location)
	require.False(t, m.inputBlockedByTerminalSize(utils.TeaRuneKeyMsg("f2")))
}
