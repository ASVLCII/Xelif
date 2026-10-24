package internal

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/ui/processbar"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestOperationVisibilityDefaultsOffAndRemembersBothChoices(t *testing.T) {
	state := filepath.Join(t.TempDir(), "visibility")
	m := defaultTestModel(t.TempDir())
	t.Cleanup(m.processBarModel.Shutdown)
	m.restoreOperationsVisibility(state)
	require.False(t, m.operations.open)
	require.NoFileExists(t, state, "reading defaults must not create or rewrite preferences")
	TeaUpdate(m, utils.TeaRuneKeyMsg("alt+o"))
	require.True(t, m.operations.open)
	data, err := os.ReadFile(state)
	require.NoError(t, err)
	require.Equal(t, "true", string(data))
	reopened := defaultTestModel(t.TempDir())
	t.Cleanup(reopened.processBarModel.Shutdown)
	reopened.restoreOperationsVisibility(state)
	require.True(t, reopened.operations.open)
	TeaUpdate(reopened, utils.TeaRuneKeyMsg("alt+o"))
	require.False(t, reopened.operations.open)
	again := defaultTestModel(t.TempDir())
	t.Cleanup(again.processBarModel.Shutdown)
	again.restoreOperationsVisibility(state)
	require.False(t, again.operations.open)
	data, err = os.ReadFile(state)
	require.NoError(t, err)
	require.Equal(t, "false", string(data))
}

func TestOperationVisibilityClosePreservesRunningWork(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	t.Cleanup(m.processBarModel.Shutdown)
	state := filepath.Join(t.TempDir(), "visibility")
	m.restoreOperationsVisibility(state)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.processBarModel.StartTask(processbar.TaskSpec{Kind: "running task"}, ctx, cancel)
	TeaUpdate(m, utils.TeaRuneKeyMsg("alt+o"))
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	require.True(t, m.operations.open, "returning from details must keep the center enabled")
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	require.False(t, m.operations.open)
	require.NoError(t, ctx.Err(), "hiding the center must not cancel the task")
	data, err := os.ReadFile(state)
	require.NoError(t, err)
	require.Equal(t, "false", string(data))
}

func TestOperationVisibilityShortcutIsolatesUnderlyingSearch(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	t.Cleanup(m.processBarModel.Shutdown)
	_ = m.globalSearchModal.OpenQuery(m.getFocusedFilePanel().Location, "needle")
	TeaUpdate(m, utils.TeaRuneKeyMsg("alt+o"))
	require.True(t, m.operations.open)
	TeaUpdate(m, utils.TeaRuneKeyMsg("q"))
	require.Equal(t, "q", m.operations.filter.Value())
	require.Equal(t, "needle", m.globalSearchModal.GetQuery())
	TeaUpdate(m, utils.TeaRuneKeyMsg("alt+o"))
	require.False(t, m.operations.open)
	require.True(t, m.globalSearchModal.IsOpen())
	require.Equal(t, "needle", m.globalSearchModal.GetQuery())
	m.globalSearchModal.Close()
}

func TestOperationVisibilityCorruptAndFailedWritesPreserveBytes(t *testing.T) {
	state := filepath.Join(t.TempDir(), "visibility")
	require.NoError(t, os.WriteFile(state, []byte("invalid existing preference"), 0o600))
	m := defaultTestModel(t.TempDir())
	t.Cleanup(m.processBarModel.Shutdown)
	m.restoreOperationsVisibility(state)
	require.False(t, m.operations.open)
	data, err := os.ReadFile(state)
	require.NoError(t, err)
	require.Equal(t, "invalid existing preference", string(data))
	directory := t.TempDir()
	require.Error(t, saveOperationsVisibility(directory, true))
	require.DirExists(t, directory)
}
