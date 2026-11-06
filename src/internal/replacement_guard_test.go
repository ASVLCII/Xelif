package internal

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/notify"
	"github.com/yorukot/superfile/src/internal/ui/processbar"
	"github.com/yorukot/superfile/src/pkg/utils"
)

type replacementGuardFixture struct {
	root        string
	source      string
	destination string
	original    string
}

func newReplacementGuardFixture(t *testing.T) replacementGuardFixture {
	t.Helper()
	root := t.TempDir()
	sourceDir := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	require.NoError(t, os.Mkdir(sourceDir, 0o700))
	require.NoError(t, os.Mkdir(destination, 0o700))
	source := filepath.Join(sourceDir, "item.txt")
	require.NoError(t, os.WriteFile(source, []byte("source data"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(destination, "original.keep"), []byte("original"), 0o600))
	return replacementGuardFixture{
		root: root, source: source, destination: destination,
		original: filepath.Join(root, "destination-original"),
	}
}

func (f replacementGuardFixture) replaceDestination(t *testing.T) {
	t.Helper()
	// Keeping the old directory alive also prevents its identity being reused.
	// On Windows, this rename checks that snapshots did not retain open handles.
	require.NoError(t, os.Rename(f.destination, f.original))
	require.NoError(t, os.Mkdir(f.destination, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(f.destination, "replacement.keep"), []byte("replacement"), 0o600))
}

func replacementGuardRequireFile(t *testing.T, path, content string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, content, string(data))
}

func (f replacementGuardFixture) requirePreserved(t *testing.T) {
	t.Helper()
	replacementGuardRequireFile(t, f.source, "source data")
	replacementGuardRequireFile(t, filepath.Join(f.original, "original.keep"), "original")
	replacementGuardRequireFile(t, filepath.Join(f.destination, "replacement.keep"), "replacement")
	for _, dir := range []string{f.original, f.destination} {
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, entries, 1, "paste must not write to %s", dir)
	}
}

// Rejection can be reported synchronously or by an operation command. Verify
// an actual error is exposed without coupling the test to a particular UI.
func requireReplacementGuardFailure(t *testing.T, m *model, msg tea.Msg) {
	t.Helper()
	switch result := msg.(type) {
	case nil:
		require.True(t, m.notifyModel.IsOpen() || m.spfError.IsOpen(), "expected an operation error")
		if m.notifyModel.IsOpen() {
			require.Equal(t, notify.NoAction, m.notifyModel.GetConfirmAction())
			require.NotEmpty(t, m.notifyModel.GetContent())
		}
	case NotifyModalUpdateMsg:
		require.True(t, result.m.IsOpen())
		require.Equal(t, notify.NoAction, result.m.GetConfirmAction())
		require.NotEmpty(t, result.m.GetContent())
	case SpfErrorModalUpdateMsg:
		require.True(t, result.m.IsOpen())
	case DeleteOperationMsg:
		require.Equal(t, processbar.Failed, result.state)
	case PasteOperationMsg:
		require.Equal(t, processbar.Failed, result.state)
	default:
		t.Fatalf("expected an operation error, got %T", msg)
	}
}

func TestReplacementGuardDeleteConfirmationPreservesReplacement(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "item.txt")
	original := filepath.Join(root, "original.txt")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))
	m := defaultTestModel(root)
	defer m.processBarModel.Shutdown()
	setFilePanelSelectedItemByLocation(t, m.getFocusedFilePanel(), path)

	applyDeleteSnapshotWarning(t, m, m.getDeleteTriggerCmd(true))
	require.NoError(t, os.Rename(path, original))
	require.NoError(t, os.WriteFile(path, []byte("replacement"), 0o600))
	cmd := confirmDeleteSnapshot(t, m)
	require.NotNil(t, cmd)
	msg := cmd()

	replacementGuardRequireFile(t, original, "original")
	replacementGuardRequireFile(t, path, "replacement")
	requireReplacementGuardFailure(t, m, msg)
}

func TestReplacementGuardContextDeletePreservesQueuedIdentity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "item.txt")
	original := filepath.Join(root, "preserved.txt")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))
	m := defaultTestModel(root)
	defer m.processBarModel.Shutdown()
	m.openContextMenu(tea.Mouse{}, 0, path)
	cmd := chooseContextAction(t, m, contextTrash)
	require.NotNil(t, cmd)
	require.NoError(t, os.Rename(path, original))
	require.NoError(t, os.WriteFile(path, []byte("replacement"), 0o600))
	msg, ok := cmd().(NotifyModalUpdateMsg)
	require.True(t, ok)
	msg.ApplyToModel(m)
	require.Equal(t, notify.NoAction, m.notifyModel.GetConfirmAction())
	require.Nil(t, m.pendingDelete)
	replacementGuardRequireFile(t, original, "original")
	replacementGuardRequireFile(t, path, "replacement")
}

func TestReplacementGuardDeleteSkipRetainsOriginalIdentities(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a.txt"), filepath.Join(root, "b.txt")
	utils.SetupFiles(t, a, b)
	m := defaultTestModel(root)
	defer m.processBarModel.Shutdown()
	setupPanelModeAndSelection(t, m, true, "", []string{a, b})
	applyDeleteSnapshotWarning(t, m, m.getDeleteTriggerCmd(true))
	require.NoError(t, os.Rename(a, filepath.Join(root, "original-a.txt")))
	require.NoError(t, os.WriteFile(a, []byte("replacement"), 0o600))
	failed, ok := confirmDeleteSnapshot(t, m)().(SpfErrorModalUpdateMsg)
	require.True(t, ok)
	failed.ApplyToModel(m)
	cmd := m.spfErrorModelOpenKey(common.Hotkeys.ConfirmTyping[0])
	require.NotNil(t, cmd)
	finished, ok := cmd().(DeleteOperationMsg)
	require.True(t, ok)
	require.Equal(t, processbar.Successful, finished.state)
	replacementGuardRequireFile(t, a, "replacement")
	require.NoFileExists(t, b)
}

func TestReplacementGuardContextFolderBeforePaste(t *testing.T) {
	for _, cut := range []bool{false, true} {
		name := commandIDCopy
		if cut {
			name = commandIDCut
		}
		t.Run(name, func(t *testing.T) {
			f := newReplacementGuardFixture(t)
			m := defaultTestModel(f.root)
			defer m.processBarModel.Shutdown()
			m.clipboard.Reset(cut)
			m.clipboard.Add(f.source)
			m.openContextMenu(tea.Mouse{}, 0, f.destination)
			require.Contains(t, m.contextMenuActionIDs(), contextPaste)

			f.replaceDestination(t)
			cmd := chooseContextAction(t, m, contextPaste)
			var msg tea.Msg
			if cmd != nil {
				msg = cmd()
			}

			f.requirePreserved(t)
			requireReplacementGuardFailure(t, m, msg)
		})
	}
}

func TestReplacementGuardQueuedPaste(t *testing.T) {
	for _, cut := range []bool{false, true} {
		name := commandIDCopy
		if cut {
			name = commandIDCut
		}
		t.Run(name, func(t *testing.T) {
			f := newReplacementGuardFixture(t)
			m := defaultTestModel(f.root)
			defer m.processBarModel.Shutdown()
			m.clipboard.Reset(cut)
			m.clipboard.Add(f.source)
			cmd := m.getPasteIntoCmd(f.destination)
			require.NotNil(t, cmd)

			f.replaceDestination(t)
			msg := cmd()
			f.requirePreserved(t)
			requireReplacementGuardFailure(t, m, msg)
		})
	}
}

func TestReplacementGuardUnchangedDestinationStillWorks(t *testing.T) {
	for _, cut := range []bool{false, true} {
		name := commandIDCopy
		if cut {
			name = commandIDCut
		}
		t.Run(name, func(t *testing.T) {
			f := newReplacementGuardFixture(t)
			m := defaultTestModel(f.root)
			defer m.processBarModel.Shutdown()
			m.clipboard.Reset(cut)
			m.clipboard.Add(f.source)
			cmd := m.getPasteIntoCmd(f.destination)
			require.NotNil(t, cmd)
			msg, ok := cmd().(PasteOperationMsg)
			require.True(t, ok)
			require.Equal(t, processbar.Successful, msg.state)
			replacementGuardRequireFile(t, filepath.Join(f.destination, "item.txt"), "source data")
			replacementGuardRequireFile(t, filepath.Join(f.destination, "original.keep"), "original")
			if cut {
				require.NoFileExists(t, f.source)
			} else {
				replacementGuardRequireFile(t, f.source, "source data")
			}
			// Snapshot and completed work should not retain directory handles.
			require.NoError(t, os.Rename(f.destination, f.original))
		})
	}
}

func TestReplacementGuardSymlinkTargetReplaced(t *testing.T) {
	f := newReplacementGuardFixture(t)
	link := filepath.Join(f.root, "destination-link")
	if err := os.Symlink(f.destination, link); err != nil {
		// Windows may lack symlink privilege or filesystem support.
		if runtime.GOOS == "windows" && (errors.Is(err, os.ErrPermission) ||
			errors.Is(err, syscall.Errno(1314)) || errors.Is(err, syscall.Errno(50)) ||
			errors.Is(err, syscall.Errno(1)) || errors.Is(err, syscall.Errno(120))) {
			t.Skipf("directory symlinks unavailable: %v", err)
		}
		require.NoError(t, err)
	}
	linkInfo, err := os.Lstat(link)
	require.NoError(t, err)
	identity, err := captureOperationIdentity(link, true)
	require.NoError(t, err)
	require.NoError(t, identity.check(link))
	m := defaultTestModel(f.root)
	defer m.processBarModel.Shutdown()
	m.clipboard.Add(f.source)
	cmd := m.getPasteIntoCmd(link)
	require.NotNil(t, cmd)

	f.replaceDestination(t)
	newLinkInfo, err := os.Lstat(link)
	require.NoError(t, err)
	require.True(t, os.SameFile(linkInfo, newLinkInfo), "the symlink entry must remain unchanged")
	msg := cmd()
	f.requirePreserved(t)
	requireReplacementGuardFailure(t, m, msg)
	// Reopening the operation is a fresh approval of the now-current target.
	fresh := m.getPasteIntoCmd(link)
	require.NotNil(t, fresh)
	completed, ok := fresh().(PasteOperationMsg)
	require.True(t, ok)
	require.Equal(t, processbar.Successful, completed.state)
	replacementGuardRequireFile(t, filepath.Join(f.destination, "item.txt"), "source data")
}
