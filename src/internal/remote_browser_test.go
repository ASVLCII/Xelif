package internal

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/remote"
	"github.com/yorukot/superfile/src/internal/ui/commandpalette"
	"github.com/yorukot/superfile/src/internal/ui/remoteviewer"
)

type fakeRemoteService struct {
	remotes  []remote.Remote
	entries  []remote.Entry
	listPath remote.RemotePath
	download string
	err      error
}

func (f *fakeRemoteService) Remotes(context.Context) ([]remote.Remote, error) {
	return append([]remote.Remote(nil), f.remotes...), f.err
}

func (f *fakeRemoteService) List(_ context.Context, location remote.RemotePath) ([]remote.Entry, error) {
	f.listPath = location
	return append([]remote.Entry(nil), f.entries...), f.err
}

func (f *fakeRemoteService) Download(_ context.Context, entry remote.Entry, localDir string) (string, error) {
	f.download = entry.Path
	return filepath.Join(localDir, entry.Name), f.err
}

func TestRemoteBrowserLoadsAndNavigatesWithoutBlockingUpdate(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	service := &fakeRemoteService{
		remotes: []remote.Remote{{Name: "cloud", Type: "s3"}},
		entries: []remote.Entry{{Remote: "cloud", Path: "photos", Name: "photos", IsDir: true}},
	}
	m.remoteClient = service
	m.remoteViewer.Open()

	msg, ok := m.loadRemoteListCmd()().(remoteOperationMsg)
	require.True(t, ok)
	m.applyRemoteOperation(msg)
	assert.Contains(t, m.remoteViewer.Render(), "cloud:")

	cmd := m.handleRemoteAction(remoteviewer.Action{
		Kind: remoteviewer.BrowseAction,
		Path: remote.RemotePath{Remote: "cloud"},
	})
	require.NotNil(t, cmd)
	msg, ok = cmd().(remoteOperationMsg)
	require.True(t, ok)
	m.applyRemoteOperation(msg)
	assert.Equal(t, "cloud:", service.listPath.String())
	assert.Contains(t, m.remoteViewer.Render(), "photos/")
}

func TestRemoteBrowserDownloadsToFocusedPanelAndShowsResult(t *testing.T) {
	directory := t.TempDir()
	m := defaultTestModel(directory)
	service := &fakeRemoteService{}
	m.remoteClient = service
	m.remoteViewer.Open()
	entry := remote.Entry{Remote: "cloud", Path: "notes.txt", Name: "notes.txt", Size: 10}
	m.remoteViewer.ShowEntries(remote.RemotePath{Remote: "cloud"}, []remote.Entry{entry})

	cmd := m.handleRemoteAction(remoteviewer.Action{Kind: remoteviewer.DownloadAction, Entry: entry})
	require.NotNil(t, cmd)
	msg, ok := cmd().(remoteOperationMsg)
	require.True(t, ok)
	m.applyRemoteOperation(msg)

	assert.Equal(t, "notes.txt", service.download)
	assert.Contains(t, m.remoteViewer.Render(), "Downloaded notes.txt")
}

func TestRemoteErrorsDoNotExposeBoundedStderrInUI(t *testing.T) {
	err := &remote.Error{
		Kind: remote.Authentication, Err: errors.New("exit status 1"), Stderr: "secret-token-value",
	}
	message := remoteUserMessage(err)
	assert.Contains(t, message, "authentication failed")
	assert.NotContains(t, message, "secret-token-value")
}

func TestRemotePaletteActionIsVisibleAndClickable(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	var item commandpalette.Item
	for _, candidate := range m.paletteItems() {
		if candidate.ID == commandIDBrowseRemote {
			item = candidate
			break
		}
	}
	assert.Equal(t, commandIDBrowseRemote, item.ID)
	assert.Equal(t, "Remote", item.Category)
	assert.False(t, item.Disabled)
}

func TestRemoteBrowserIgnoresStaleCompletion(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	m.remoteViewer.Open()
	m.remoteReqID = 2
	m.applyRemoteOperation(remoteOperationMsg{
		reqID: 1, kind: remoteLoadRemotes, remotes: []remote.Remote{{Name: "stale", Type: "s3"}},
	})
	assert.NotContains(t, m.remoteViewer.Render(), "stale:")
}
