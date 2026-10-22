package filepanel

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

func TestPanelRefreshCadenceBoundaries(t *testing.T) {
	lastRead := time.Unix(1700000000, 0)
	for _, test := range []struct {
		count   int
		focused bool
		delay   time.Duration
	}{
		{0, true, time.Second},
		{1, true, time.Second},
		{99, true, time.Second},
		{100, true, time.Second},
		{199, true, time.Second},
		{200, true, 2 * time.Second},
		{299, true, 2 * time.Second},
		{300, true, 3 * time.Second},
		{1000, true, 3 * time.Second},
		{0, false, 3 * time.Second},
		{1, false, 3 * time.Second},
	} {
		t.Run(fmt.Sprintf("entries_%d/focused_%t", test.count, test.focused), func(t *testing.T) {
			panel := newPanelRefreshFixture(t.TempDir(), test.focused)
			panel.element = make([]Element, test.count)
			for index := range panel.element {
				panel.element[index].Location = filepath.Join(panel.Location, fmt.Sprintf("file-%d", index))
			}
			panel.LastTimeGetElement = lastRead
			assert.True(t, panel.shouldSkipPanelUpdate(lastRead))
			assert.True(t, panel.shouldSkipPanelUpdate(lastRead.Add(test.delay-time.Nanosecond)))
			assert.False(t, panel.shouldSkipPanelUpdate(lastRead.Add(test.delay)))
			assert.False(t, panel.shouldSkipPanelUpdate(lastRead.Add(test.delay+time.Nanosecond)))
		})
	}
}

func TestPanelRefreshInitialAndInvalidPanelsAreImmediate(t *testing.T) {
	root := t.TempDir()
	writePanelRefreshFile(t, root, "alpha.txt", "alpha")
	panel := newPanelRefreshFixture(root, true)
	assert.False(t, panel.shouldSkipPanelUpdate(time.Now()))
	panel.UpdateElementsIfNeeded(false, false)
	require.Equal(t, []string{"alpha.txt"}, panelRefreshNames(&panel))

	lastRead := pinPanelRefreshWindow(&panel)
	panel.cursor = -1
	assert.False(t, panel.shouldSkipPanelUpdate(lastRead.Add(time.Millisecond)))
	panel.UpdateElementsIfNeeded(false, false)
	assert.Equal(t, 0, panel.GetCursor())
	assert.NotEqual(t, lastRead, panel.LastTimeGetElement)
}

func TestPanelRefreshStableDirectoryPreservesCursorAndSelection(t *testing.T) {
	root := t.TempDir()
	writePanelRefreshFile(t, root, "alpha.txt", "alpha")
	writePanelRefreshFile(t, root, "beta.txt", "beta")
	panel := newPanelRefreshFixture(root, true)
	panel.UpdateElementsIfNeeded(false, false)
	require.True(t, panel.FocusIndex(1))
	require.True(t, panel.SelectRangeTo(1))
	anchor := panel.selectionAnchor
	selected := panel.GetSelectedLocationsSortedAsVisible()
	lastRead := pinPanelRefreshWindow(&panel)
	writePanelRefreshFile(t, root, "gamma.txt", "gamma")

	for range 10 {
		panel.UpdateElementsIfNeeded(false, false)
	}
	assert.Equal(t, lastRead, panel.LastTimeGetElement)
	assert.Equal(t, []string{"alpha.txt", "beta.txt"}, panelRefreshNames(&panel))
	assert.Equal(t, 1, panel.GetCursor())
	assert.Equal(t, anchor, panel.selectionAnchor)
	assert.Equal(t, selected, panel.GetSelectedLocationsSortedAsVisible())

	panel.UpdateElementsIfNeeded(true, false)
	assert.NotEqual(t, lastRead, panel.LastTimeGetElement)
	assert.Equal(t, []string{"alpha.txt", "beta.txt", "gamma.txt"}, panelRefreshNames(&panel))
	assert.Equal(t, 1, panel.GetCursor())
	assert.Equal(t, anchor, panel.selectionAnchor)
	assert.Equal(t, selected, panel.GetSelectedLocationsSortedAsVisible())

	require.NoError(t, os.Remove(filepath.Join(root, "beta.txt")))
	panel.UpdateElementsIfNeeded(true, false)
	assert.Empty(t, panel.selectionAnchor)
}

func TestPanelRefreshEmptyAndNoMatchesAreThrottled(t *testing.T) {
	for _, query := range []string{"", "missing"} {
		t.Run("query_"+query, func(t *testing.T) {
			root := t.TempDir()
			if query != "" {
				writePanelRefreshFile(t, root, "alpha.txt", "alpha")
			}
			panel := newPanelRefreshFixture(root, true)
			panel.SearchBar.SetValue(query)
			panel.UpdateElementsIfNeeded(false, false)
			require.Empty(t, panelRefreshNames(&panel))
			lastRead := pinPanelRefreshWindow(&panel)
			writePanelRefreshFile(t, root, "missing.txt", "new file")
			panel.UpdateElementsIfNeeded(false, false)
			assert.Equal(t, lastRead, panel.LastTimeGetElement)
			assert.Empty(t, panelRefreshNames(&panel))

			// Exercise the expired fallback without sleeping or depending on a tick.
			panel.LastTimeGetElement = time.Unix(1, 0)
			panel.UpdateElementsIfNeeded(false, false)
			assert.Equal(t, []string{"missing.txt"}, panelRefreshNames(&panel))
		})
	}
}

func TestPanelRefreshExpiredFallbackObservesCreationAndDeletion(t *testing.T) {
	root := t.TempDir()
	writePanelRefreshFile(t, root, "alpha.txt", "alpha")
	writePanelRefreshFile(t, root, "beta.txt", "beta")
	panel := newPanelRefreshFixture(root, true)
	panel.UpdateElementsIfNeeded(false, false)
	require.True(t, panel.SelectRangeTo(0))
	require.Equal(t, filepath.Join(root, "alpha.txt"), panel.selectionAnchor)

	writePanelRefreshFile(t, root, "gamma.txt", "gamma")
	require.NoError(t, os.Remove(filepath.Join(root, "alpha.txt")))
	// No watcher notification and no force flag: an expired ordinary update
	// must still reconcile external filesystem changes and selection anchors.
	panel.LastTimeGetElement = time.Unix(1, 0)
	panel.UpdateElementsIfNeeded(false, false)
	assert.Equal(t, []string{"beta.txt", "gamma.txt"}, panelRefreshNames(&panel))
	assert.Empty(t, panel.selectionAnchor)
	assert.NoError(t, panel.ValidateCursorAndRenderIndex())
}

func TestPanelRefreshSearchChangesAndClearingAreImmediate(t *testing.T) {
	for _, focused := range []bool{true, false} {
		t.Run(fmt.Sprintf("focused_%t", focused), func(t *testing.T) {
			root := t.TempDir()
			writePanelRefreshFile(t, root, "alpha.txt", "alpha")
			writePanelRefreshFile(t, root, "beta.bin", "beta")
			panel := newPanelRefreshFixture(root, focused)
			panel.UpdateElementsIfNeeded(false, false)
			for _, test := range []struct {
				query string
				want  []string
			}{
				{"alpha", []string{"alpha.txt"}},
				{"beta", []string{"beta.bin"}},
				{"no-such-entry", nil},
				{"", []string{"alpha.txt", "beta.bin"}},
				{"alpha", []string{"alpha.txt"}},
				{"", []string{"alpha.txt", "beta.bin"}},
			} {
				lastRead := pinPanelRefreshWindow(&panel)
				panel.SearchBar.SetValue(test.query)
				panel.SearchBar.Blur()
				panel.UpdateElementsIfNeeded(false, false)
				assert.Equal(t, test.want, panelRefreshNames(&panel), "query %q", test.query)
				assert.NotEqual(t, lastRead, panel.LastTimeGetElement)
			}
		})
	}
}

func TestPanelRefreshSortAndDotFileChangesAreImmediate(t *testing.T) {
	root := t.TempDir()
	writePanelRefreshFile(t, root, "alpha.txt", "longer content")
	writePanelRefreshFile(t, root, "beta.bin", "b")
	writePanelRefreshFile(t, root, ".hidden", "")
	panel := newPanelRefreshFixture(root, true)
	panel.UpdateElementsIfNeeded(false, false)
	require.Equal(t, []string{"alpha.txt", "beta.bin"}, panelRefreshNames(&panel))

	lastRead := pinPanelRefreshWindow(&panel)
	panel.SortKind = sortmodel.SortBySize
	panel.UpdateElementsIfNeeded(false, false)
	assert.Equal(t, []string{"beta.bin", "alpha.txt"}, panelRefreshNames(&panel))
	assert.NotEqual(t, lastRead, panel.LastTimeGetElement)

	lastRead = pinPanelRefreshWindow(&panel)
	panel.ToggleReverseSort()
	panel.UpdateElementsIfNeeded(false, false)
	assert.Equal(t, []string{"alpha.txt", "beta.bin"}, panelRefreshNames(&panel))
	assert.NotEqual(t, lastRead, panel.LastTimeGetElement)

	lastRead = pinPanelRefreshWindow(&panel)
	panel.UpdateElementsIfNeeded(false, true)
	assert.Equal(t, []string{"alpha.txt", "beta.bin", ".hidden"}, panelRefreshNames(&panel))
	assert.NotEqual(t, lastRead, panel.LastTimeGetElement)
	lastRead = pinPanelRefreshWindow(&panel)
	panel.UpdateElementsIfNeeded(false, false)
	assert.Equal(t, []string{"alpha.txt", "beta.bin"}, panelRefreshNames(&panel))
	assert.NotEqual(t, lastRead, panel.LastTimeGetElement)
}

func TestPanelRefreshCaseSensitiveSortChangesAreImmediate(t *testing.T) {
	original := common.Config.CaseSensitiveSort
	t.Cleanup(func() { common.Config.CaseSensitiveSort = original })
	common.Config.CaseSensitiveSort = false
	root := t.TempDir()
	writePanelRefreshFile(t, root, "apple", "a")
	writePanelRefreshFile(t, root, "Zebra", "z")
	panel := newPanelRefreshFixture(root, true)
	panel.UpdateElementsIfNeeded(false, false)
	require.Equal(t, []string{"apple", "Zebra"}, panelRefreshNames(&panel))
	lastRead := pinPanelRefreshWindow(&panel)
	common.Config.CaseSensitiveSort = true
	panel.UpdateElementsIfNeeded(false, false)
	assert.Equal(t, []string{"Zebra", "apple"}, panelRefreshNames(&panel))
	assert.NotEqual(t, lastRead, panel.LastTimeGetElement)
}

func TestPanelRefreshNavigationAndTargetAreImmediate(t *testing.T) {
	for _, focused := range []bool{true, false} {
		t.Run(fmt.Sprintf("focused_%t", focused), func(t *testing.T) {
			panel := newPanelRefreshFixture(t.TempDir(), focused)
			panel.UpdateElementsIfNeeded(false, false)
			lastRead := pinPanelRefreshWindow(&panel)
			target := t.TempDir()
			writePanelRefreshFile(t, target, "alpha.txt", "alpha")
			writePanelRefreshFile(t, target, "beta.txt", "beta")
			require.NoError(t, panel.UpdateCurrentFilePanelDir(target))
			panel.UpdateElementsIfNeeded(false, false)
			assert.Equal(t, []string{"alpha.txt", "beta.txt"}, panelRefreshNames(&panel))
			assert.NotEqual(t, lastRead, panel.LastTimeGetElement)

			lastRead = pinPanelRefreshWindow(&panel)
			panel.TargetFile = "beta.txt"
			panel.UpdateElementsIfNeeded(false, false)
			assert.Equal(t, 1, panel.GetCursor())
			assert.Empty(t, panel.TargetFile)
			assert.NotEqual(t, lastRead, panel.LastTimeGetElement)
		})
	}
}

// Public-API fixtures keep these benchmarks usable against the saved old source
// via a Go overlay; idle and forced paths use the same directory and code path.
func BenchmarkPanelRefresh(b *testing.B) {
	for _, count := range []int{0, 25, 99, 300} {
		b.Run(fmt.Sprintf("entries_%d", count), func(b *testing.B) {
			root := b.TempDir()
			for index := range count {
				writePanelRefreshFile(b, root, fmt.Sprintf("file-%03d.txt", index), "data")
			}
			for _, force := range []bool{false, true} {
				b.Run(fmt.Sprintf("force_%t", force), func(b *testing.B) {
					panel := newPanelRefreshFixture(root, true)
					panel.UpdateElementsIfNeeded(true, false)
					b.ReportAllocs()
					for b.Loop() {
						// Keep the workload within one fallback interval even during
						// long benchmark runs. Both variants pay this clock cost.
						panel.LastTimeGetElement = time.Now()
						panel.UpdateElementsIfNeeded(force, false)
					}
				})
			}
		})
	}
}

func newPanelRefreshFixture(location string, focused bool) Model {
	panel := New(location, focused, "", sortmodel.SortByName, false)
	panel.UpdateDimensions(80, 24)
	return panel
}

func writePanelRefreshFile(tb testing.TB, root, name, content string) {
	tb.Helper()
	require.NoError(tb, os.WriteFile(filepath.Join(root, name), []byte(content), 0o600))
}

func panelRefreshNames(panel *Model) []string {
	var names []string
	for _, element := range panel.element {
		names = append(names, element.Name)
	}
	return names
}

func pinPanelRefreshWindow(panel *Model) time.Time {
	// Boundary tests use explicit timestamps. Public-API behavior tests pin a
	// future read time so elapsed wall time cannot accidentally expire the window.
	lastRead := time.Now().Add(time.Hour)
	panel.LastTimeGetElement = lastRead
	return lastRead
}
