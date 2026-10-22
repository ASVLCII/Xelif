package filepanel

import (
	"path/filepath"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"

	"github.com/yorukot/superfile/src/internal/gitstatus"
)

func TestGitStatusBadgeAndReplacement(t *testing.T) {
	panel := Model{Location: t.TempDir()}
	path := filepath.Join(panel.Location, "report.txt")
	panel.SetGitStates(map[string]gitstatus.State{path: gitstatus.Modified})
	assert.Equal(t, gitstatus.Modified, panel.GitState(path))
	assert.Equal(t, "[M] ", panel.gitStatusBadge(path))

	panel.SetGitStates(map[string]gitstatus.State{})
	assert.Equal(t, gitstatus.Clean, panel.GitState(path))
	assert.Empty(t, panel.gitStatusBadge(path))
	assert.Equal(t, 4, ansi.StringWidth("[M] "))
}
