package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/gitstatus"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

func newGitRefreshFixture(t testing.TB, count int) (*model, string) {
	t.Helper()
	root := t.TempDir()
	for index := range count {
		path := filepath.Join(root, fmt.Sprintf("file-%05d", index))
		require.NoError(t, os.WriteFile(path, []byte("contents"), 0o600))
	}
	panel := filepanel.New(root, true, "", sortmodel.SortByName, false)
	panel.UpdateElementsIfNeeded(true, false)
	m := &model{gitClient: gitstatus.New(gitstatus.Options{})}
	m.fileModel.FilePanels = []filepanel.Model{panel}
	return m, root
}

func TestGitRefreshGateKeepsPendingContentsAndRejectsStaleReplies(t *testing.T) {
	m, root := newGitRefreshFixture(t, 1)
	output, err := exec.Command("git", "init", "--quiet", root).CombinedOutput()
	require.NoError(t, err, "%s", output)
	first := m.refreshGitStatusCmd(true)
	require.NotNil(t, first)
	stale := first().(gitStatusUpdateMsg)
	firstRequest := m.gitStatusReqID
	require.Nil(t, m.refreshGitStatusCmd(false))
	require.Equal(t, firstRequest, m.gitStatusReqID)

	newFile := filepath.Join(root, "new-file")
	require.NoError(t, os.WriteFile(newFile, []byte("new"), 0o600))
	m.fileModel.FilePanels[0].UpdateElementsIfNeeded(true, false)
	require.Nil(t, m.refreshGitStatusCmd(false), "entry changes retain the normal refresh interval")
	m.gitStatusLast = time.Now().Add(-gitStatusRefreshInterval)
	next := m.refreshGitStatusCmd(false)
	require.NotNil(t, next)
	current := next().(gitStatusUpdateMsg)
	require.Empty(t, current.errors)
	require.Equal(t, gitstatus.Untracked, current.states[root][newFile])
	m.applyGitStatusUpdate(current)
	require.Equal(t, gitstatus.Untracked, m.fileModel.FilePanels[0].GitState(newFile))
	m.applyGitStatusUpdate(stale)
	require.Equal(t, gitstatus.Untracked, m.fileModel.FilePanels[0].GitState(newFile))
	require.NotNil(t, m.refreshGitStatusCmd(true), "explicit refresh bypasses the time gate")

	other := t.TempDir()
	m.fileModel.FilePanels[0].Location = other
	require.NotNil(t, m.refreshGitStatusCmd(false), "active location changes bypass the time gate")
}

func BenchmarkGitRefreshThrottled(b *testing.B) {
	for _, count := range []int{100, 10000} {
		b.Run(fmt.Sprintf("entries_%d", count), func(b *testing.B) {
			m, _ := newGitRefreshFixture(b, count)
			m.refreshGitStatusCmd(true)
			m.gitStatusLast = time.Now().Add(time.Hour)
			b.ReportAllocs()
			for b.Loop() {
				if m.refreshGitStatusCmd(false) != nil {
					b.Fatal("throttled refresh unexpectedly produced a command")
				}
			}
		})
	}
}

// Include the Git refresh gate in the existing synchronous explorer input
// fixture. Returned background commands and terminal painting are excluded.
func BenchmarkExplorerRootUpdateWithGitStatus(b *testing.B) {
	if os.Getenv("XELIF_REFRESH_CORPUS") == "" {
		root := b.TempDir()
		prepareExplorerUpdateCorpus(b, root)
		b.Setenv("XELIF_REFRESH_CORPUS", root)
	}
	for _, count := range []int{50, 2000} {
		b.Run(fmt.Sprintf("entries_%d", count), func(b *testing.B) {
			m := explorerUpdateBenchmarkModel(b, count, false)
			m.gitClient = gitstatus.New(gitstatus.Options{})
			m.refreshGitStatusCmd(true)
			m.gitStatusLast = time.Now().Add(time.Hour)
			keys := [...]tea.KeyPressMsg{{Code: tea.KeyDown}, {Code: tea.KeyUp}}
			step := 0
			b.ReportAllocs()
			for b.Loop() {
				m.Update(keys[step&1])
				step++
			}
			panel := m.getFocusedFilePanel()
			if panel.GetCursor() != step&1 || panel.ElemCount() != count {
				b.Fatal("Git-enabled input changed the fixture navigation state")
			}
		})
	}
}
