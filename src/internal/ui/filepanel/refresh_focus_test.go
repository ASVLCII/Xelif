package filepanel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRefreshPreservesFocusedPathWhenEarlierEntriesChange(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"bravo.txt", "charlie.txt", "delta.txt"} {
		writePanelRefreshFile(t, root, name, name)
	}
	panel := newPanelRefreshFixture(root, true)
	panel.UpdateElementsIfNeeded(true, false)
	require.True(t, panel.FocusIndex(1))
	focused := panel.GetFocusedItem().Location

	writePanelRefreshFile(t, root, "alpha.txt", "new")
	panel.UpdateElementsIfNeeded(true, false)
	require.Equal(t, focused, panel.GetFocusedItem().Location,
		"adding an earlier entry must not redirect the user's next action")

	require.NoError(t, os.Remove(filepath.Join(root, "bravo.txt")))
	panel.UpdateElementsIfNeeded(true, false)
	require.Equal(t, focused, panel.GetFocusedItem().Location)
	require.NoError(t, panel.ValidateCursorAndRenderIndex())

	panel.TargetFile = "delta.txt"
	panel.UpdateElementsIfNeeded(true, false)
	require.Equal(t, "delta.txt", panel.GetFocusedItem().Name,
		"explicit focus requests must override refresh preservation")
}

func TestRefreshDoesNotRestorePreviousDirectoryFocus(t *testing.T) {
	root := t.TempDir()
	next := filepath.Join(root, "next")
	require.NoError(t, os.Mkdir(next, 0o700))
	writePanelRefreshFile(t, root, "bravo.txt", "old")
	writePanelRefreshFile(t, next, "alpha.txt", "new")
	panel := newPanelRefreshFixture(root, true)
	panel.UpdateElementsIfNeeded(true, false)
	require.True(t, panel.FocusIndex(1))
	require.NoError(t, panel.UpdateCurrentFilePanelDir(next))
	panel.UpdateElementsIfNeeded(true, false)
	require.Equal(t, filepath.Join(next, "alpha.txt"), panel.GetFocusedItem().Location)
	require.NoError(t, panel.ValidateCursorAndRenderIndex())
}
