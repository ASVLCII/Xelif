package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPluginCompletionCannotReplaceAnotherWorkspaceViewer(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	view := m.operationSubmissionView()
	_ = m.newWorkspace(t.TempDir())
	m.textViewer.Open("Current workspace", "Keep this content")
	before := m.textViewer.Render()
	cancelled := false
	m.pluginCommandCancel = func() { cancelled = true }
	m.applyPluginCommandResult(pluginCommandFinishedMsg{reqID: m.pluginCommandReqID, view: view})
	require.True(t, cancelled, "completed task context is released globally")
	require.Nil(t, m.pluginCommandCancel)
	require.Equal(t, before, m.textViewer.Render())
	require.False(t, m.notifyModel.IsOpen())
}
