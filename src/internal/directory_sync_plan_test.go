package internal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/dircompare"
)

func TestDirectorySyncPreviewRejectsChangedSource(t *testing.T) {
	for _, scenario := range []string{"added", "modified", "replaced", "destination-arrival"} {
		t.Run(scenario, func(t *testing.T) {
			source, destination := t.TempDir(), t.TempDir()
			path := filepath.Join(source, "reviewed.txt")
			require.NoError(t, os.WriteFile(path, []byte("reviewed"), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(source, "existing.txt"), []byte("source"), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(destination, "existing.txt"), []byte("keep"), 0o600))
			m := defaultTestModel(source, destination)
			completeDirectorySyncPreview(t, m)
			switch scenario {
			case "added":
				require.NoError(t, os.WriteFile(filepath.Join(source, "unreviewed.txt"), []byte("new"), 0o600))
			case "modified", "replaced":
				info, err := os.Stat(path)
				require.NoError(t, err)
				contents := "modified"
				if scenario == "replaced" {
					require.NoError(t, os.Rename(path, filepath.Join(t.TempDir(), "original")))
					contents = "reviewed"
				}
				require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
				require.NoError(t, os.Chtimes(path, info.ModTime(), info.ModTime()))
			case "destination-arrival":
				require.NoError(
					t,
					os.WriteFile(filepath.Join(destination, "reviewed.txt"), []byte("late arrival"), 0o600),
				)
			}
			cmd := m.applyPendingDirectorySync()
			require.NotNil(t, cmd)
			msg, ok := cmd().(directorySyncMsg)
			require.True(t, ok)
			require.Error(t, msg.err, "changed trees require a new preview")
			require.Empty(t, msg.result.Created)
			m.applyDirectorySync(msg)
			require.Nil(t, m.pendingSync, "a failed approval cannot be reused")
			require.NoFileExists(t, filepath.Join(destination, "unreviewed.txt"))
			if scenario == "destination-arrival" {
				assertDirectorySyncContents(t, filepath.Join(destination, "reviewed.txt"), "late arrival")
			} else {
				require.NoFileExists(t, filepath.Join(destination, "reviewed.txt"))
			}
			assertDirectorySyncContents(t, filepath.Join(destination, "existing.txt"), "keep")
		})
	}
}

func TestDirectorySyncApprovalIsConsumedBeforeCommandRuns(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(source, "reviewed.txt"), []byte("reviewed"), 0o600))
	m := defaultTestModel(source, destination)
	completeDirectorySyncPreview(t, m)
	cmd := m.applyPendingDirectorySync()
	require.NotNil(t, cmd)
	require.Nil(t, m.pendingSync)
	require.Nil(t, m.applyPendingDirectorySync(), "the same approval cannot queue a second copy")
	msg, ok := cmd().(directorySyncMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)
	assertDirectorySyncContents(t, filepath.Join(destination, "reviewed.txt"), "reviewed")
}

func TestDirectorySyncCancelInvalidatesApprovalAndDelayedPreview(t *testing.T) {
	m := defaultTestModel(t.TempDir(), t.TempDir())
	completeDirectorySyncPreview(t, m)
	cmd := m.previewDirectorySync()
	require.NotNil(t, cmd)
	require.Nil(t, m.pendingSync, "starting another operation invalidates the old approval")
	msg, ok := cmd().(directoryCompareMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)
	m.cancelDirectoryCompare()
	m.applyDirectoryCompare(msg)
	require.Nil(t, m.pendingSync, "a result arriving after cancellation must not restore approval")
	require.Nil(t, m.applyPendingDirectorySync())
}

func TestDirectorySyncCancelledCommandCannotWriteOrConsumeNewPreview(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(source, "reviewed.txt"), []byte("reviewed"), 0o600))
	m := defaultTestModel(source, destination)
	completeDirectorySyncPreview(t, m)
	oldCommand := m.applyPendingDirectorySync()
	require.NotNil(t, oldCommand)
	completeDirectorySyncPreview(t, m)
	newApproval := m.pendingSync
	msg, ok := oldCommand().(directorySyncMsg)
	require.True(t, ok)
	require.ErrorIs(t, msg.err, context.Canceled)
	m.applyDirectorySync(msg)
	require.Same(t, newApproval, m.pendingSync)
	require.NoFileExists(t, filepath.Join(destination, "reviewed.txt"))
}

func TestDirectorySyncApprovalRejectsChangedPanelContext(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(map[bool]string{false: "navigated", true: "reversed"}[reverse], func(t *testing.T) {
			source, destination := t.TempDir(), t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(source, "reviewed.txt"), []byte("reviewed"), 0o600))
			m := defaultTestModel(source, destination)
			completeDirectorySyncPreview(t, m)
			if reverse {
				m.fileModel.FocusedPanelIndex = 1
			} else {
				require.NoError(t, m.updateCurrentFilePanelDir(t.TempDir()))
			}
			require.Nil(t, m.applyPendingDirectorySync())
			require.Nil(t, m.pendingSync)
			require.Contains(t, ansi.Strip(m.textViewer.Render()), "preview again")
			require.NoFileExists(t, filepath.Join(destination, "reviewed.txt"))
		})
	}
}

func TestDirectorySyncPartialResultRefreshesWithoutReusingApproval(t *testing.T) {
	for _, stale := range []bool{false, true} {
		t.Run(map[bool]string{false: "current", true: "stale"}[stale], func(t *testing.T) {
			source, destination := t.TempDir(), t.TempDir()
			m := defaultTestModel(source, destination)
			completeDirectorySyncPreview(t, m)
			approval := m.pendingSync
			panel := &m.fileModel.FilePanels[1]
			panel.LastTimeGetElement = time.Now().Add(time.Hour)
			require.NoError(t, os.WriteFile(filepath.Join(destination, "completed.txt"), []byte("retained"), 0o600))
			reqID := m.dirCompareReqID
			if stale {
				reqID--
			}
			m.applyDirectorySync(directorySyncMsg{
				reqID:  reqID,
				result: dircompare.CopyResult{Created: []string{"completed.txt"}},
				err: &dircompare.PartialCopyError{
					Created: []string{"completed.txt"},
					Err:     errors.New("late collision"),
				},
			})
			require.NotEqual(t, -1, panel.FindElementIndexByName("completed.txt"))
			assertDirectorySyncContents(t, filepath.Join(destination, "completed.txt"), "retained")
			if stale {
				require.Same(t, approval, m.pendingSync)
			} else {
				require.Nil(t, m.pendingSync)
				require.Contains(t, ansi.Strip(m.textViewer.Render()), "retained")
			}
		})
	}
}

func TestDirectorySyncFailureReplacesProgressWithVisibleCreationAudit(t *testing.T) {
	for _, created := range []int{0, 1} {
		t.Run(map[int]string{0: "no-writes", 1: "partial-copy"}[created], func(t *testing.T) {
			m := defaultTestModel(t.TempDir(), t.TempDir())
			m.textViewer.Open("Copying missing items", "still copying")
			result := dircompare.CopyResult{}
			if created > 0 {
				result.Created = []string{"completed.txt"}
			}
			m.applyDirectorySync(
				directorySyncMsg{reqID: m.dirCompareReqID, result: result, err: errors.New("source changed")},
			)
			rendered := ansi.Strip(m.textViewer.Render())
			require.Contains(t, rendered, "Copy-missing operation failed")
			require.Contains(t, rendered, "source changed")
			if created > 0 {
				require.Contains(t, rendered, "Created 1")
				require.Contains(t, rendered, "retained")
			} else {
				require.Contains(t, rendered, "Created 0")
			}
			require.NotContains(t, rendered, "still copying")
			require.False(t, m.notifyModel.IsOpen(), "the failure must not leave a hidden notification")
		})
	}
}

func TestDirectoryCompareFailureReplacesProgressWithoutChangingNewerOverlay(t *testing.T) {
	m := defaultTestModel(t.TempDir(), t.TempDir())
	m.textViewer.Open("Directory comparison", "still comparing")
	m.applyDirectoryCompare(directoryCompareMsg{reqID: m.dirCompareReqID, err: errors.New("source vanished")})
	rendered := ansi.Strip(m.textViewer.Render())
	require.Contains(t, rendered, "Directory comparison failed")
	require.Contains(t, rendered, "source vanished")
	require.NotContains(t, rendered, "still comparing")
	require.False(t, m.notifyModel.IsOpen())
	m.textViewer.Open("Newer operation", "keep this result")
	newer := m.textViewer.Render()
	m.applyDirectoryCompare(directoryCompareMsg{reqID: m.dirCompareReqID - 1, err: errors.New("stale compare failure")})
	m.applyDirectorySync(directorySyncMsg{reqID: m.dirCompareReqID - 1, err: errors.New("stale copy failure")})
	require.Equal(t, newer, m.textViewer.Render())
	require.False(t, m.notifyModel.IsOpen())
}

func completeDirectorySyncPreview(t *testing.T, m *model) {
	t.Helper()
	cmd := m.previewDirectorySync()
	require.NotNil(t, cmd)
	msg, ok := cmd().(directoryCompareMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)
	m.applyDirectoryCompare(msg)
	require.NotNil(t, m.pendingSync)
}

func assertDirectorySyncContents(t *testing.T, path, want string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, want, string(contents))
}
