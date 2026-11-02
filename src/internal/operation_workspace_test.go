package internal

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/batchrename"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/remote"
	"github.com/yorukot/superfile/src/internal/ui/archiveviewer"
)

// The submitted plan must reconcile Undo/catalog globally even after the origin
// workspace is gone, and must never consume a newer preview or replace the
// active workspace's viewer.
func TestOperationBatchRenameCompletionReconcilesAfterWorkspaceSwitch(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	first := filepath.Join(root, "first.txt")
	second := filepath.Join(root, "second.txt")
	require.NoError(t, os.WriteFile(first, []byte("first"), 0o600))
	require.NoError(t, os.WriteFile(second, []byte("second"), 0o600))
	newerFile := filepath.Join(other, "newer.txt")
	require.NoError(t, os.WriteFile(newerFile, []byte("newer"), 0o600))

	m := dailyDriverTestModel(root)
	m.catalogModel = loadCatalog(filepath.Join(t.TempDir(), "catalog.json"))
	require.NoError(t, m.catalogModel.CreateCollection("work", []string{first, second}))

	panel := m.getFocusedFilePanel()
	panel.ChangeFilePanelMode()
	panel.SetSelectedAll([]string{first, second})
	_, cmd, err := m.executeBatchRenameAction(common.BatchRenameAction{
		Operation: common.BatchRenamePrefix, Replacement: "done-",
	})
	require.NoError(t, err)
	require.Nil(t, cmd)
	require.NotNil(t, m.pendingBatchRename)

	_, applyCmd, err := m.executeBatchRenameAction(common.BatchRenameAction{Operation: common.BatchRenameApply})
	require.NoError(t, err)
	require.NotNil(t, applyCmd)

	_ = m.newWorkspace(other)
	require.Len(t, m.workspaces, 2)
	require.Nil(t, m.pendingBatchRename, "switching must clear the origin workspace preview")

	newerPlan, err := batchrename.NewPlan([]string{newerFile}, batchrename.Rule{Suffix: "-new"})
	require.NoError(t, err)
	m.pendingBatchRename = &newerPlan
	newer := m.pendingBatchRename

	m.textViewer.Open("Current workspace", "keep this content")
	before := m.textViewer.Render()

	msg, ok := applyCmd().(batchRenameOperationMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)
	m.applyBatchRenameOperation(msg)

	for _, name := range []string{"done-first.txt", "done-second.txt"} {
		_, statErr := os.Stat(filepath.Join(root, name))
		require.NoError(t, statErr, "the submitted rename must still complete globally")
	}
	require.Same(t, newer, m.pendingBatchRename, "a newer preview must survive the old completion")
	require.Equal(t, before, m.textViewer.Render(), "the old completion must not replace the active view")
	require.NotNil(t, m.lastUndo, "global Undo must be recorded for the completed rename")

	paths, err := m.catalogModel.GetCollection("work")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		filepath.Join(root, "done-first.txt"),
		filepath.Join(root, "done-second.txt"),
	}, paths, "catalog paths must follow the completed rename")

	require.NoError(t, m.undoLastFileOperation())
	for _, path := range []string{first, second} {
		_, statErr := os.Stat(path)
		require.NoError(t, statErr)
	}
}

// Stale completions from A must not open or replace B's viewer/modal.
func TestOperationStaleCompletionsLeaveActiveWorkspaceUntouched(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	origin := m.operationSubmissionView()

	archivePath := writeLifecycleArchive(t, map[string]string{"member.txt": "archive content"}, zip.Store)
	require.NoError(t, m.openArchive(archivePath))
	t.Cleanup(func() { _ = m.archiveViewer.Close() })
	previewCmd := m.handleArchiveAction(archiveviewer.Action{
		Kind: archiveviewer.PreviewAction, Member: "member.txt",
	})
	require.NotNil(t, previewCmd)
	preview, ok := previewCmd().(archiveOperationMsg)
	require.True(t, ok)
	require.NoError(t, preview.err)

	require.NotNil(t, m.newWorkspace(t.TempDir()))
	m.textViewer.Open("Current workspace", "keep this content")
	before := m.textViewer.Render()

	m.applyArchiveOperation(preview)
	require.Equal(t, before, m.textViewer.Render())
	require.False(t, m.notifyModel.IsOpen())

	_ = m.applyGitOperation(gitOperationMsg{title: gitDiffTitle, content: "stale diff", view: origin})
	require.Equal(t, before, m.textViewer.Render())
	_ = m.applyGitOperation(gitOperationMsg{err: errors.New("stale git failure"), view: origin})
	require.False(t, m.notifyModel.IsOpen())
	require.Equal(t, before, m.textViewer.Render())

	m.applyRemoteOperation(remoteOperationMsg{
		reqID: m.remoteReqID, kind: remoteLoadRemotes, view: origin,
	})
	require.False(t, m.remoteViewer.IsOpen(), "a stale listing must not reopen the remote browser")
	m.applyRemoteOperation(remoteOperationMsg{
		reqID: m.remoteReqID, kind: remoteLoadRemotes,
		err: errors.New("stale remote failure"), view: origin,
	})
	require.False(t, m.notifyModel.IsOpen())
	require.Equal(t, before, m.textViewer.Render())
}

// Explicit extraction is a mutation: closing the origin archive view on switch
// must cancel only its UI context, not the submitted extraction.
func TestOperationArchiveExtractionSurvivesWorkspaceSwitch(t *testing.T) {
	destination := t.TempDir()
	archivePath := writeLifecycleArchive(t, map[string]string{"member.txt": "archive content"}, zip.Store)
	m := dailyDriverTestModel(destination)
	require.NoError(t, m.openArchive(archivePath))
	t.Cleanup(func() { _ = m.archiveViewer.Close() })

	cmd := m.handleArchiveAction(archiveviewer.Action{
		Kind: archiveviewer.ExtractAction, Member: "member.txt",
	})
	require.NotNil(t, cmd)

	require.NotNil(t, m.newWorkspace(t.TempDir()))
	m.textViewer.Open("Current workspace", "keep this content")
	before := m.textViewer.Render()

	msg, ok := cmd().(archiveOperationMsg)
	require.True(t, ok)
	require.NoError(t, msg.err, "explicit extraction must not be cancelled by closing the origin view")
	content, err := os.ReadFile(filepath.Join(destination, "member.txt"))
	require.NoError(t, err)
	require.Equal(t, "archive content", string(content))

	m.applyArchiveOperation(msg)
	require.Equal(t, before, m.textViewer.Render(), "the extraction completion must not focus another workspace")
	require.False(t, m.notifyModel.IsOpen())
}

type operationWorkspaceRemote struct {
	started chan struct{}
	release chan struct{}
}

func (f *operationWorkspaceRemote) Remotes(context.Context) ([]remote.Remote, error) {
	return nil, nil
}

func (f *operationWorkspaceRemote) List(context.Context, remote.RemotePath) ([]remote.Entry, error) {
	return nil, nil
}

func (f *operationWorkspaceRemote) Download(
	ctx context.Context,
	entry remote.Entry,
	destination string,
) (string, error) {
	close(f.started)
	select {
	case <-f.release:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	localPath := filepath.Join(destination, "downloaded.txt")
	if err := os.WriteFile(localPath, []byte("payload"), 0o600); err != nil {
		return "", err
	}
	return localPath, nil
}

// A download is a mutation: a workspace switch or a new listing must not cancel
// the in-flight transfer, and its completion must not focus another workspace.
func TestOperationRemoteDownloadSurvivesWorkspaceSwitch(t *testing.T) {
	root := t.TempDir()
	m := dailyDriverTestModel(root)
	client := &operationWorkspaceRemote{started: make(chan struct{}), release: make(chan struct{})}
	m.remoteClient = client
	t.Cleanup(m.processBarModel.Shutdown)
	t.Cleanup(func() {
		select {
		case <-client.release:
		default:
			close(client.release)
		}
	})

	cmd := m.downloadRemoteCmd(remote.Entry{Remote: "test", Path: "file.txt"})
	require.NotNil(t, cmd)

	result := make(chan remoteOperationMsg, 1)
	go func() {
		msg, ok := cmd().(remoteOperationMsg)
		if ok {
			result <- msg
		}
	}()
	select {
	case <-client.started:
	case <-time.After(5 * time.Second):
		t.Fatal("download did not start")
	}

	require.NotNil(t, m.newWorkspace(t.TempDir()))
	listCmd := m.loadRemoteListCmd()
	require.NotNil(t, listCmd)
	_ = listCmd() // A new listing must not cancel the in-flight download.

	close(client.release)
	var msg remoteOperationMsg
	select {
	case msg = <-result:
	case <-time.After(time.Second):
		t.Fatal("download did not finish")
	}
	require.NoError(t, msg.err)
	require.Equal(t, remoteDownloadFile, msg.kind)
	require.FileExists(t, filepath.Join(root, "downloaded.txt"))

	m.textViewer.Open("Current workspace", "keep this content")
	before := m.textViewer.Render()
	m.applyRemoteOperation(msg)
	require.Equal(t, before, m.textViewer.Render())
	require.False(t, m.notifyModel.IsOpen())
}

// Legacy models have no workspace state; a zero-value origin must still present
// same-view completions exactly as before.
func TestOperationLegacyZeroViewStillShowsGitCompletion(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	require.Empty(t, m.workspaces)

	_ = m.applyGitOperation(gitOperationMsg{title: gitDiffTitle, content: "legacy diff"})
	require.True(t, m.textViewer.IsOpen())
	require.Contains(t, m.textViewer.Render(), "legacy diff")
}
