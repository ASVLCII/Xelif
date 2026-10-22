package sidebar

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/pkg/utils"
)

func refreshTestSidebar(t *testing.T) (Model, string, string) {
	t.Helper()
	root := t.TempDir()
	first, second := filepath.Join(root, "first"), filepath.Join(root, "second")
	utils.SetupDirectories(t, first, second)
	pinned := NewPinnedFileManager(filepath.Join(root, "pinned.json"))
	recent := NewRecentFileManager(filepath.Join(root, "recent.json"))
	require.NoError(t, pinned.Save([]directory{{Location: first, Name: "first"}}))
	return Model{
		pinnedMgr: &pinned, recentMgr: &recent,
		sections: []string{utils.SidebarSectionPinned, utils.SidebarSectionRecent}, height: 6,
	}, first, second
}

func TestSidebarRoutineRefreshReusesSnapshotUntilPoll(t *testing.T) {
	s, first, second := refreshTestSidebar(t)
	now := time.Now()
	s.updateDirectoriesIfNeeded(now)
	require.Equal(t, first, s.directories[0].Location)
	// Simulate another process editing the pinned file. Input events must not
	// read it repeatedly, but the bounded fallback must eventually see it.
	require.NoError(t, s.pinnedMgr.Save([]directory{{Location: second, Name: "second"}}))
	s.updateDirectoriesIfNeeded(now.Add(directoryRefreshInterval - time.Nanosecond))
	require.Equal(t, first, s.directories[0].Location)
	require.Equal(t, now, s.lastRefresh)
	s.updateDirectoriesIfNeeded(now.Add(directoryRefreshInterval))
	require.Equal(t, second, s.directories[0].Location)
}

func TestSidebarQueryAndClearRefreshImmediately(t *testing.T) {
	s, first, second := refreshTestSidebar(t)
	require.NoError(t, s.pinnedMgr.Save([]directory{
		{Location: first, Name: "first"}, {Location: second, Name: "second"},
	}))
	now := time.Now()
	s.updateDirectoriesIfNeeded(now)
	s.searchBar.SetValue("second")
	s.updateDirectoriesIfNeeded(now)
	require.Len(t, s.directories, 1)
	require.Equal(t, second, s.directories[0].Location)
	s.searchBar.SetValue("")
	s.updateDirectoriesIfNeeded(now)
	require.Len(t, s.directories, 2)
}

func TestSidebarMutationsInvalidateSnapshotImmediately(t *testing.T) {
	s, first, second := refreshTestSidebar(t)
	now := time.Now()
	s.updateDirectoriesIfNeeded(now)
	require.NoError(t, s.TogglePinnedDirectory(second))
	s.updateDirectoriesIfNeeded(now)
	require.Len(t, s.directories, 2)
	require.NoError(t, s.AddRecent(first))
	s.updateDirectoriesIfNeeded(now)
	require.Len(t, s.directories, 4) // two pinned, divider, one recent
	s.searchBar.SetValue("first")
	s.updateDirectoriesIfNeeded(now)
	s.PinnedItemRename()
	s.rename.SetValue("renamed")
	s.ConfirmSidebarRename()
	s.updateDirectoriesIfNeeded(now)
	require.Len(t, s.directories, 1, "renamed pin must leave the filtered results immediately")
	require.Equal(t, utils.SidebarSectionRecent, s.directories[0].Section)
}

func TestSidebarPollingKeepsIndependentViewport(t *testing.T) {
	s, first, second := refreshTestSidebar(t)
	require.NoError(t, s.pinnedMgr.Save([]directory{
		{Location: first, Name: "first"}, {Location: second, Name: "second"},
	}))
	now := time.Now()
	s.updateDirectoriesIfNeeded(now)
	s.Scroll(1)
	index, offset := s.renderIndex, s.renderOffset
	s.updateDirectoriesIfNeeded(now.Add(directoryRefreshInterval))
	require.Equal(t, index, s.renderIndex)
	require.Equal(t, offset, s.renderOffset)
	require.Zero(t, s.cursor)
}

func TestSidebarExplicitRefreshBypassesThrottle(t *testing.T) {
	s, _, second := refreshTestSidebar(t)
	s.UpdateDirectories()
	require.NoError(t, s.pinnedMgr.Save([]directory{{Location: second, Name: "second"}}))
	s.UpdateDirectories()
	require.Equal(t, second, s.directories[0].Location)
}

func TestDisabledSidebarRefreshDoesNotAccessManagers(t *testing.T) {
	s := Model{disabled: true}
	s.UpdateDirectoriesIfNeeded()
	s.UpdateDirectories()
	require.True(t, s.lastRefresh.IsZero())
}
