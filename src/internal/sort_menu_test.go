package internal

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

func TestExplorerSortToolbarAndDirectionApplyImmediately(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a-small.txt"), []byte("a"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "z-large.txt"), []byte("large file"), 0600))
	m := defaultTestModel(root)
	TeaUpdate(m, tea.WindowSizeMsg{Width: 120, Height: 28})
	panel := m.getFocusedFilePanel()
	panel.ApplySort(sortmodel.SortByName, false)
	setFilePanelSelectedItemByName(t, panel, "z-large.txt")
	focused := panel.GetFocusedItem().Location
	open := func() {
		x, buttonY := renderedLabelPosition(t, m.mainComponentsRender(), panel.SortActionLabel())
		TeaUpdate(m, clickAt(x+1, buttonY, tea.MouseLeft))
		require.True(t, m.sortModal.IsOpen())
		left, top := m.sortMenuPosition()
		require.Equal(t, x, left)
		require.Equal(t, buttonY+1, top)
	}
	choose := func(row int) {
		left, top := m.sortMenuPosition()
		TeaUpdate(m, clickAt(left+4, top+1+row, tea.MouseLeft))
		require.False(t, m.sortModal.IsOpen())
		require.Equal(t, focused, panel.GetFocusedItem().Location)
	}
	open()
	choose(3) // Size, independent of persisted enum order.
	require.Equal(t, sortmodel.SortBySize, panel.SortKind)
	require.Equal(t, "a-small.txt", panel.GetElementAtIdx(0).Name)
	for range 2 {
		open()
		choose(7) // Descending is explicit and idempotent.
		require.Equal(t, sortmodel.SortBySize, panel.SortKind)
		require.True(t, panel.SortReversed)
		require.Equal(t, "z-large.txt", panel.GetElementAtIdx(0).Name)
	}
	open()
	choose(6)
	require.False(t, panel.SortReversed)
	require.Equal(t, sortmodel.SortBySize, panel.SortKind)
}

func TestExplorerSortSeparatorAndOutsideClicksDoNotReachFiles(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	TeaUpdate(m, tea.WindowSizeMsg{Width: 80, Height: 22})
	m.openSortMenu()
	left, top := m.sortMenuPosition()
	TeaUpdate(m, clickAt(left+4, top+6, tea.MouseLeft))
	require.True(t, m.sortModal.IsOpen(), "separator is not a sort action")
	TeaUpdate(m, clickAt(0, 0, tea.MouseLeft))
	require.False(t, m.sortModal.IsOpen())
	m.openSortMenu()
	m.sortOptionsKey("esc")
	require.False(t, m.sortModal.IsOpen())
}

func TestExplorerSortDropdownStaysWithinResizedScreen(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	for _, width := range []int{60, 80, 120} {
		TeaUpdate(m, tea.WindowSizeMsg{Width: width, Height: 22})
		m.openSortMenu()
		x, y := m.sortMenuPosition()
		require.GreaterOrEqual(t, x, 0)
		require.GreaterOrEqual(t, y, 0)
		require.LessOrEqual(t, x+lipgloss.Width(m.sortModal.Render()), width)
		require.LessOrEqual(t, y+lipgloss.Height(m.sortModal.Render()), 22)
		m.sortModal.Close()
	}
}
