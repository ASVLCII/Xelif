package internal

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/ui/processbar"
)

// This is an ownership/retention regression, not an OS memory or physical
// rendering benchmark. Native resource measurements remain a separate gate.
func TestWorkspaceHundredCyclesRetainOnlyActiveWatchTargets(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	m := dailyDriverTestModel(root)
	tasks := m.fileModel.Tasks
	originalID := m.workspaces[0].State.ID
	const cycles = 100
	for range cycles {
		stale := m.workspaceCommand(func() tea.Msg { return "obsolete workspace reply" })()
		_ = m.newWorkspace(other)
		require.Equal(t, []string{other}, m.currentFolderWatchPaths())
		require.Len(t, m.fileModel.FilePanels, 1)
		_, next := m.Update(stale)
		require.Nil(t, next)
		_ = m.closeWorkspace()
		require.Equal(t, originalID, m.workspaces[m.activeWorkspace].State.ID)
		require.Equal(t, []string{root}, m.currentFolderWatchPaths())
		require.Len(t, m.workspaces, 1)
		require.Same(t, tasks, m.fileModel.Tasks)
		require.LessOrEqual(t, len(m.closedWorkspaces), maxClosedWorkspaces)
	}
	require.Len(t, m.closedWorkspaces, maxClosedWorkspaces)
	require.Equal(t, uint64(cycles*2), m.workspaceGeneration)
}

func TestFinalWorkspaceCloseKeepsGlobalOperationAndNotice(t *testing.T) {
	root := t.TempDir()
	m := dailyDriverTestModel(root)
	t.Cleanup(m.processBarModel.Shutdown)
	process := m.processBarModel.StartTask(processbar.TaskSpec{Kind: "Fixture operation"}, nil, nil)
	_ = m.closeWorkspace()
	require.Len(t, m.workspaces, 1)
	require.Equal(t, root, m.getFocusedFilePanel().Location)
	require.Contains(t, m.workspaceNotice, "operations continue")
	_, exists := m.processBarModel.GetByID(process.ID)
	require.True(t, exists)
	require.True(t, m.processBarModel.HasRunningProcesses())
}
