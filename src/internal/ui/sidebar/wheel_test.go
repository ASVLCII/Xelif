package sidebar

import (
	"path/filepath"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestRefreshKeepsScrolledViewportWhenEntriesAreUnchanged(t *testing.T) {
	root := t.TempDir()
	first, last := filepath.Join(root, "first"), filepath.Join(root, "last")
	utils.SetupDirectories(t, first, last)
	recent := NewRecentFileManager(filepath.Join(root, "recent.json"))
	require.NoError(t, recent.Save([]directory{{Location: first, Name: "first"}, {Location: last, Name: "last"}}))
	pinned := NewPinnedFileManager(filepath.Join(root, "pinned.json"))
	s := Model{pinnedMgr: &pinned, recentMgr: &recent, sections: []string{utils.SidebarSectionRecent}, height: 6}
	s.UpdateDirectories()
	s.Scroll(1)
	require.Equal(t, 1, s.renderIndex)
	s.UpdateDirectories()
	require.Equal(t, 1, s.renderIndex, "routine refresh must not recenter the cursor")
	require.Zero(t, s.cursor)
}

func TestWheelScrollsRowsWithoutChangingSelection(t *testing.T) {
	s := testModel(0, 0, 9, nil, []directory{
		{Name: "first", Location: "/first"}, recentDividerDir,
		{Name: "second", Location: "/second"}, diskDividerDir,
		{Name: "last-disk", Location: "/last"},
	})
	s.width = 24
	s.Scroll(-1)
	require.Zero(t, s.renderIndex)
	s.Scroll(1)
	require.Equal(t, 1, s.renderIndex)
	require.Zero(t, s.renderOffset)
	s.Scroll(1)
	require.Equal(t, 1, s.renderIndex)
	require.Equal(t, 1, s.renderOffset, "headings scroll one row too")
	require.Zero(t, s.cursor)
	for range 30 {
		s.Scroll(1)
	}
	view := ansi.Strip(s.Render(true, "/first"))
	require.Contains(t, view, "last-disk")
	require.Zero(t, s.cursor, "wheel must not change the selected directory")
	beforeIndex, beforeOffset := s.renderIndex, s.renderOffset
	s.Scroll(1)
	require.Equal(t, beforeIndex, s.renderIndex)
	require.Equal(t, beforeOffset, s.renderOffset)
	for range 30 {
		s.Scroll(-1)
	}
	require.Zero(t, s.renderIndex)
	require.Zero(t, s.renderOffset)
}

func TestScrolledPartialHeadingHitTargetsAndResize(t *testing.T) {
	s := testModel(0, 1, 8, nil, []directory{
		{Name: "first", Location: "/first"}, diskDividerDir,
		{Name: "last", Location: "/last"},
	})
	s.renderOffset = 1
	_, ok := s.FocusDirectoryAtY(4, false)
	require.False(t, ok, "heading title is not a directory")
	path, ok := s.FocusDirectoryAtY(6, false)
	require.True(t, ok)
	require.Equal(t, "/last", path)
	s.SetHeight(20)
	require.Zero(t, s.renderIndex)
	require.Zero(t, s.renderOffset)
	s.ListUp()
	require.Zero(t, s.renderOffset)
}
