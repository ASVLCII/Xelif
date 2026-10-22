package remoteviewer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/remote"
)

func TestRemoteViewerNavigationIsExplicit(t *testing.T) {
	m := New()
	m.Open()
	assert.True(t, m.IsOpen())
	assert.True(t, m.IsLoading())
	assert.Equal(t, NoAction, m.Enter().Kind)

	m.ShowRemotes([]remote.Remote{{Name: "photos", Type: "s3"}})
	action := m.Enter()
	assert.Equal(t, BrowseAction, action.Kind)
	assert.Equal(t, "photos:", action.Path.String())

	m.ShowEntries(action.Path, []remote.Entry{
		{Remote: "photos", Path: "albums", Name: "albums", IsDir: true},
		{Remote: "photos", Path: "cover.jpg", Name: "cover.jpg", Size: 42},
	})
	assert.Equal(t, BrowseAction, m.Enter().Kind)
	m.Move(1)
	download := m.Enter()
	assert.Equal(t, DownloadAction, download.Kind)
	assert.Equal(t, "cover.jpg", download.Entry.Name)
	assert.Equal(t, LoadRemotesAction, m.Back().Kind)
}

func TestRemoteViewerParentAndRetryActions(t *testing.T) {
	m := New()
	m.Open()
	location, err := remote.ParsePath("cloud:one/two")
	require.NoError(t, err)
	m.ShowEntries(location, nil)

	back := m.Back()
	assert.Equal(t, BrowseAction, back.Kind)
	assert.Equal(t, "cloud:one", back.Path.String())

	m.ShowError("Authentication failed")
	retry := m.Retry()
	assert.Equal(t, BrowseAction, retry.Kind)
	assert.Equal(t, location, retry.Path)
	assert.Contains(t, m.Render(), "Authentication failed")
}

func TestRemoteViewerMouseSelectsAndDoubleClicks(t *testing.T) {
	m := New()
	m.Open()
	m.ShowRemotes([]remote.Remote{{Name: "one", Type: "local"}, {Name: "two", Type: "s3"}})
	now := time.Now()
	assert.Equal(t, NoAction, m.HandleClick(4, entryStartRow+1, now).Kind)
	action := m.HandleClick(4, entryStartRow+1, now.Add(100*time.Millisecond))
	assert.Equal(t, BrowseAction, action.Kind)
	assert.Equal(t, "two", action.Path.Remote)

	closeHit := actionHits()[3]
	assert.Equal(t, CloseAction, m.HandleClick(closeHit.start, actionRow, now).Kind)
}

func TestRemoteViewerRenderShowsClickableActionsAndEmptyState(t *testing.T) {
	m := New()
	m.SetDimensions(60, 12)
	m.Open()
	m.ShowRemotes(nil)
	view := m.Render()
	for _, text := range []string{"Remote files", "[Open]", "[Download]", "[Back / Retry]", "[Close]"} {
		assert.Contains(t, view, text)
	}
	assert.Contains(t, view, "Nothing here")
}
