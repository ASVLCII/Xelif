package filepanel

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/yorukot/superfile/src/internal/gitstatus"
)

func (m *Model) SetGitStates(states map[string]gitstatus.State) {
	m.gitStates = make(map[string]gitstatus.State, len(states))
	for path, state := range states {
		if state != gitstatus.Clean {
			m.gitStates[gitPathKey(path)] = state
		}
	}
}

func (m *Model) GitState(path string) gitstatus.State {
	if state, ok := m.gitStates[gitPathKey(path)]; ok {
		return state
	}
	return gitstatus.Clean
}

func (m *Model) gitStatusBadge(path string) string {
	var symbol string
	switch m.GitState(path) {
	case gitstatus.Clean:
		return ""
	case gitstatus.Modified:
		symbol = "M"
	case gitstatus.Added:
		symbol = "A"
	case gitstatus.Deleted:
		symbol = "D"
	case gitstatus.Renamed:
		symbol = "R"
	case gitstatus.Untracked:
		symbol = "?"
	case gitstatus.Ignored:
		symbol = "I"
	case gitstatus.Conflict:
		symbol = "!"
	default:
		return ""
	}
	return "[" + symbol + "] "
}

func gitPathKey(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}
