package internal

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
)

func executeWorkspaceFixtureCommand(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	if batch, ok := cmd().(tea.BatchMsg); ok {
		for _, child := range batch {
			executeWorkspaceFixtureCommand(t, child)
		}
	}
}

func TestNamedWorkspaceLoadReturnsWatchSynchronizationCommand(t *testing.T) {
	for _, viaPalette := range []bool{false, true} {
		t.Run(map[bool]string{false: "prompt", true: "palette"}[viaPalette], func(t *testing.T) {
			saved, other := t.TempDir(), t.TempDir()
			m := dailyDriverTestModel(saved)
			m.gitClient = nil
			m.sessionStore = testSessionStore(t)
			require.NoError(t, m.saveWorkspace("project"))
			require.NoError(t, m.updateCurrentFilePanelDir(other))
			watcher := newFakeFolderWatcher()
			m.folderWatcher = watcher
			var cmd tea.Cmd
			if viaPalette {
				var handled bool
				cmd, handled = m.applyStoredPaletteCommand(commandIDWorkspaceLoadPrefix + "project")
				require.True(t, handled)
			} else {
				_, next, err := m.executeWorkspaceAction(
					common.WorkspaceAction{Operation: common.WorkspaceLoad, Name: "project"},
				)
				require.NoError(t, err)
				cmd = next
			}
			require.NotNil(t, cmd)
			require.Empty(t, watcher.syncCalls, "filesystem watcher must not run before the returned command")
			executeWorkspaceFixtureCommand(t, cmd)
			require.Equal(t, [][]string{{saved}}, watcher.syncCalls)
			require.Len(t, m.workspaces, 2, "the existing workspace is not replaced")
		})
	}
}
