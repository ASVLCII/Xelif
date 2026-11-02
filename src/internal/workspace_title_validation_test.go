package internal

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/session"
)

func TestWorkspaceGeneratedTitlesFitDurableByteLimit(t *testing.T) {
	for _, base := range []string{strings.Repeat("界", 200), "e" + strings.Repeat("\u0301", 200), "line\nbreak", "  "} {
		title := boundedWorkspaceTitle(base, " (2)")
		require.NotEmpty(t, title)
		require.True(t, utf8.ValidString(title))
		require.LessOrEqual(t, len(title), session.MaxWorkspaceTitleBytes)
		require.Empty(t, workspaceRenameValidation(title))
		require.True(t, strings.HasSuffix(title, " (2)"))
	}
}

func TestWorkspaceInvalidRenameErrorStaysInVisibleForm(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	id := m.workspaces[0].State.ID
	_ = m.beginWorkspaceRename(id)
	m.workspaceNameInput.CharLimit = 0 // Exercise validation independently of the input widget's own limit.
	before := m.workspaces[0].State.Title
	for _, invalid := range []string{"", strings.Repeat("界", 100)} {
		m.workspaceNameInput.SetValue(invalid)
		m.commitWorkspaceRename()
		require.True(t, m.workspaceRenaming)
		require.NotEmpty(t, m.workspaceRenameError)
		require.False(t, m.notifyModel.IsOpen(), "error must not open a hidden second modal")
		require.Equal(t, before, m.workspaces[0].State.Title)
	}
	m.workspaceNameInput.SetValue("Working files")
	m.commitWorkspaceRename()
	require.False(t, m.workspaceRenaming)
	require.Equal(t, "Working files", m.workspaces[0].State.Title)
}

func TestWorkspaceTitleValidationRejectsControlCharacters(t *testing.T) {
	require.NotEmpty(t, workspaceRenameValidation("control\x01name"))
	require.NotEmpty(t, workspaceRenameValidation(string([]byte{0xff})))
}

func TestWorkspaceOpenLimitPreservesCurrentAndClosedState(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	current := m.workspaces[0]
	m.workspaces = make([]workspaceRuntime, session.MaxWorkspaces)
	m.workspaces[0] = current
	m.closedWorkspaces = []workspaceRuntime{current}
	require.Nil(t, m.newWorkspace(t.TempDir()))
	require.Nil(t, m.reopenWorkspace())
	_, err := m.loadWorkspace("saved")
	require.ErrorContains(t, err, "close an open workspace")
	require.Len(t, m.workspaces, session.MaxWorkspaces)
	require.Len(t, m.closedWorkspaces, 1)
	require.Equal(t, current.State.ID, m.workspaces[0].State.ID)
}
