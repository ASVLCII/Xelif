package filepanel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/session"
)

func TestWorkspaceSelectionRefusesReplacementIdentity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "selected.txt")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))
	panel := newTabTestPanel(t, root)
	panel.UpdateElementsIfNeeded(true, false)
	panel.SetSelected(path)
	view := panel.SnapshotView()
	require.Contains(t, view.Selected, path)
	require.NoError(t, os.Rename(path, filepath.Join(root, "original.txt")))
	require.NoError(t, os.WriteFile(path, []byte("replacement"), 0o600))
	panel.RestoreView(view, false)
	require.False(t, panel.CheckSelected(path))
}

func TestWorkspaceRetainsUnavailablePathsAndShowsNotice(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "offline-drive")
	panel := RestoreWorkspace(
		session.PanelState{Tabs: []session.TabState{{Location: missing, BackHistory: []string{missing}}}},
		t.TempDir(),
		true,
	)
	panel.WorkspaceChrome = true
	panel.SetWidth(80)
	panel.SetHeight(24)
	panel.UpdateElementsIfNeeded(true, false)
	require.Equal(t, missing, panel.Location)
	require.Equal(t, missing, panel.SnapshotSession().Tabs[0].BackHistory[0])
	require.Contains(t, panel.Render(true), "Folder unavailable")
}

func TestWorkspacePreservesFilteredSelectionAndOrder(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.txt")
	second := filepath.Join(root, "second.txt")
	for _, path := range []string{first, second} {
		require.NoError(t, os.WriteFile(path, nil, 0o600))
	}
	panel := newTabTestPanel(t, root)
	panel.UpdateElementsIfNeeded(true, false)
	panel.SetSelected(second)
	panel.SetSelected(first)
	panel.SearchBar.SetValue("first")
	panel.UpdateElementsIfNeeded(true, false)
	panel.selectionAnchor = first
	require.Equal(t, 1, panel.ElemCount())
	view := panel.SnapshotView()
	panel.ResetSelected()
	panel.RestoreView(view, false)
	require.True(t, panel.CheckSelected(first))
	require.True(t, panel.CheckSelected(second))
	require.Equal(t, second, panel.GetFirstSelectedLocation())
	require.Equal(t, first, panel.selectionAnchor)
}
