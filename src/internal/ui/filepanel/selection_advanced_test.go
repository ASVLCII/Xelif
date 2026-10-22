package filepanel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

func newSelectionTestPanel(t *testing.T, location string, reversed bool) Model {
	t.Helper()
	originalHotkeys := common.Hotkeys
	common.Hotkeys.SearchBar = []string{"/"}
	t.Cleanup(func() {
		common.Hotkeys = originalHotkeys //nolint:reassign // Restore the process-wide test configuration.
	})
	return New(location, true, "", sortmodel.SortByName, reversed)
}

func selectionTestElements() []Element {
	return []Element{
		{Name: "a-dir", Location: "/tmp/a-dir", Directory: true},
		{Name: "b.txt", Location: "/tmp/b.txt"},
		{Name: "c-dir", Location: "/tmp/c-dir", Directory: true},
		{Name: "d.txt", Location: "/tmp/d.txt"},
		{Name: "e.txt", Location: "/tmp/e.txt"},
	}
}

func TestSelectRangeToSelectsInclusiveForwardRangeFromInitialCursor(t *testing.T) {
	panel := testModel(1, 0, 12, SelectMode, selectionTestElements())

	assert.True(t, panel.SelectRangeTo(4))
	assert.Equal(t, map[string]int{
		"/tmp/b.txt": 1,
		"/tmp/c-dir": 2,
		"/tmp/d.txt": 3,
		"/tmp/e.txt": 4,
	}, panel.selected)
	assert.Equal(t, 1, panel.GetCursor())
}

func TestSelectRangeToSelectsInclusiveBackwardRange(t *testing.T) {
	panel := testModel(4, 0, 12, SelectMode, selectionTestElements())

	assert.True(t, panel.SelectRangeTo(1))
	assert.Equal(t, []string{
		"/tmp/b.txt",
		"/tmp/c-dir",
		"/tmp/d.txt",
		"/tmp/e.txt",
	}, panel.GetSelectedLocationsSortedAsVisible())
}

func TestSelectRangeToKeepsAnchorAndExistingSelectionOrder(t *testing.T) {
	panel := testModel(2, 0, 12, SelectMode, selectionTestElements())
	panel.SetSelected("/outside/first")
	panel.SetSelected("/tmp/d.txt")

	require.True(t, panel.SelectRangeTo(4))
	assert.Equal(t, 2, panel.selected["/tmp/d.txt"], "an existing selection must not be reordered")
	require.True(t, panel.SelectRangeTo(0))
	assert.Equal(t, map[string]int{
		"/outside/first": 1,
		"/tmp/d.txt":     2,
		"/tmp/c-dir":     3,
		"/tmp/e.txt":     4,
		"/tmp/a-dir":     5,
		"/tmp/b.txt":     6,
	}, panel.selected)
}

func TestSelectRangeToKeepsInitialAnchorWhenCursorMoves(t *testing.T) {
	panel := testModel(2, 0, 12, SelectMode, selectionTestElements())
	require.True(t, panel.SelectRangeTo(2))
	panel.SetCursorPosition(4)

	require.True(t, panel.SelectRangeTo(3))

	assert.Equal(t, []string{"/tmp/c-dir", "/tmp/d.txt"}, panel.GetSelectedLocationsSortedAsVisible())
	assert.False(t, panel.CheckSelected("/tmp/e.txt"))
}

func TestSelectRangeToRejectsInvalidStateWithoutCreatingAnchor(t *testing.T) {
	tests := []struct {
		name  string
		panel Model
		index int
	}{
		{name: "empty", panel: testModel(0, 0, 12, SelectMode, nil), index: 0},
		{name: "negative target", panel: testModel(0, 0, 12, SelectMode, selectionTestElements()), index: -1},
		{name: "target past end", panel: testModel(0, 0, 12, SelectMode, selectionTestElements()), index: 5},
		{name: "invalid cursor", panel: testModel(8, 0, 12, SelectMode, selectionTestElements()), index: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, tt.panel.SelectRangeTo(tt.index))
			assert.Empty(t, tt.panel.selected)
			assert.Empty(t, tt.panel.selectionAnchor)
		})
	}
}

func TestInvertVisibleSelectionPreservesHiddenSelectionsAndOrder(t *testing.T) {
	panel := testModel(0, 0, 12, SelectMode, selectionTestElements()[1:4])
	panel.SetSelected("/hidden.txt")
	panel.SetSelected("/tmp/b.txt")
	panel.SetSelected("/tmp/d.txt")

	panel.InvertVisibleSelection()

	assert.Equal(t, map[string]int{
		"/hidden.txt": 1,
		"/tmp/c-dir":  4,
	}, panel.selected)
}

func TestInvertVisibleSelectionIsSafeForEmptyView(t *testing.T) {
	panel := testModel(0, 0, 12, SelectMode, nil)
	panel.SetSelected("/hidden.txt")

	panel.InvertVisibleSelection()

	assert.Equal(t, map[string]int{"/hidden.txt": 1}, panel.selected)
}

func TestSelectVisibleWhereAndTypeWrappersUseCurrentVisibleElements(t *testing.T) {
	t.Run("predicate", func(t *testing.T) {
		panel := testModel(0, 0, 12, SelectMode, selectionTestElements()[1:])
		panel.SelectVisibleWhere(func(element Element) bool {
			return element.Name == "b.txt" || element.Name == "e.txt"
		})

		assert.Equal(t, map[string]int{"/tmp/b.txt": 1, "/tmp/e.txt": 2}, panel.selected)
	})

	t.Run("nil predicate", func(t *testing.T) {
		panel := testModel(0, 0, 12, SelectMode, selectionTestElements())
		panel.SelectVisibleWhere(nil)
		assert.Empty(t, panel.selected)
	})

	t.Run("files", func(t *testing.T) {
		panel := testModel(0, 0, 12, SelectMode, selectionTestElements())
		panel.SelectVisibleFiles()
		assert.Equal(t, []string{"/tmp/b.txt", "/tmp/d.txt", "/tmp/e.txt"},
			panel.GetSelectedLocationsSortedAsVisible())
	})

	t.Run("directories", func(t *testing.T) {
		panel := testModel(0, 0, 12, SelectMode, selectionTestElements())
		panel.SelectVisibleDirectories()
		assert.Equal(t, []string{"/tmp/a-dir", "/tmp/c-dir"},
			panel.GetSelectedLocationsSortedAsVisible())
	})
}

func TestSelectRangeToUsesFilteredAndSortedVisibleOrder(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha.txt", "bravo.txt", "charlie.txt", "delta.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(name), 0o600))
	}
	panel := newSelectionTestPanel(t, root, true)
	panel.SearchBar.SetValue("l")
	panel.UpdateElementsIfNeeded(true, false)
	require.Equal(t, []string{"delta.txt", "charlie.txt", "alpha.txt"}, elementNames(panel.element))
	panel.SetCursorPosition(0)

	require.True(t, panel.SelectRangeTo(1))
	assert.Equal(t, []string{filepath.Join(root, "delta.txt"), filepath.Join(root, "charlie.txt")},
		panel.GetSelectedLocationsSortedAsVisible())
	assert.False(t, panel.CheckSelected(filepath.Join(root, "bravo.txt")))
}

func TestSelectionAnchorTracksIdentityAcrossSortRefresh(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha.txt", "bravo.txt", "charlie.txt", "delta.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(name), 0o600))
	}
	panel := newSelectionTestPanel(t, root, false)
	panel.UpdateElementsIfNeeded(true, false)
	panel.SetCursorPosition(1)
	require.True(t, panel.SelectRangeTo(2))

	panel.ToggleReverseSort()
	panel.UpdateElementsIfNeeded(true, false)
	require.Equal(t, []string{"delta.txt", "charlie.txt", "bravo.txt", "alpha.txt"}, elementNames(panel.element))
	require.True(t, panel.SelectRangeTo(3))

	assert.Equal(t, []string{
		filepath.Join(root, "charlie.txt"),
		filepath.Join(root, "bravo.txt"),
		filepath.Join(root, "alpha.txt"),
	}, panel.GetSelectedLocationsSortedAsVisible())
	assert.False(t, panel.CheckSelected(filepath.Join(root, "delta.txt")))
}

func TestElementRefreshClearsAnchorWhenIdentityDisappears(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha.txt")
	bravo := filepath.Join(root, "bravo.txt")
	require.NoError(t, os.WriteFile(alpha, []byte("alpha"), 0o600))
	require.NoError(t, os.WriteFile(bravo, []byte("bravo"), 0o600))
	panel := newSelectionTestPanel(t, root, false)
	panel.UpdateElementsIfNeeded(true, false)
	panel.SetCursorPosition(0)
	require.True(t, panel.SelectRangeTo(1))
	require.NotEmpty(t, panel.selectionAnchor)

	require.NoError(t, os.Remove(alpha))
	panel.UpdateElementsIfNeeded(true, false)

	assert.Empty(t, panel.selectionAnchor)
}

func TestSelectionAnchorResetTransitions(t *testing.T) {
	t.Run("explicit clear", func(t *testing.T) {
		panel := testModel(0, 0, 12, SelectMode, selectionTestElements())
		require.True(t, panel.SelectRangeTo(1))
		panel.ClearSelectionAnchor()
		assert.Empty(t, panel.selectionAnchor)
	})

	t.Run("reset selected", func(t *testing.T) {
		panel := testModel(0, 0, 12, SelectMode, selectionTestElements())
		require.True(t, panel.SelectRangeTo(1))
		panel.ResetSelected()
		assert.Empty(t, panel.selectionAnchor)
	})

	t.Run("leave select mode", func(t *testing.T) {
		panel := testModel(0, 0, 12, SelectMode, selectionTestElements())
		require.True(t, panel.SelectRangeTo(1))
		panel.ChangeFilePanelMode()
		assert.Equal(t, BrowserMode, panel.PanelMode)
		assert.Empty(t, panel.selectionAnchor)
	})

	t.Run("directory change", func(t *testing.T) {
		root := t.TempDir()
		child := filepath.Join(root, "child")
		require.NoError(t, os.Mkdir(child, 0o700))
		panel := newSelectionTestPanel(t, root, false)
		panel.element = selectionTestElements()
		require.True(t, panel.SelectRangeTo(1))

		require.NoError(t, panel.UpdateCurrentFilePanelDir(child))
		assert.Empty(t, panel.selectionAnchor)
	})
}

func elementNames(elements []Element) []string {
	names := make([]string, len(elements))
	for index := range elements {
		names[index] = elements[index].Name
	}
	return names
}
