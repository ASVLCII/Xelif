package filepanel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/session"
	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

func prepareSessionRestoreTest(t *testing.T) {
	t.Helper()
	originalHotkeys := common.Hotkeys
	common.Hotkeys.SearchBar = []string{"/"}
	t.Cleanup(func() {
		common.Hotkeys = originalHotkeys //nolint:reassign // Restore the process-wide test configuration.
	})
}

func TestSessionRoundTripPreservesTabsHistoryAndSort(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")
	third := filepath.Join(root, "third")
	fourth := filepath.Join(root, "fourth")
	for _, location := range []string{first, second, third, fourth} {
		require.NoError(t, os.Mkdir(location, 0o755))
	}

	panel := newTabTestPanel(t, root)
	require.NoError(t, panel.UpdateCurrentFilePanelDir(first))
	require.NoError(t, panel.UpdateCurrentFilePanelDir(second))
	require.NoError(t, panel.Back())
	require.NoError(t, panel.NewTab(third))
	require.NoError(t, panel.UpdateCurrentFilePanelDir(fourth))
	require.NoError(t, panel.ActivateTab(0))
	panel.SortKind = sortmodel.SortByNatural
	panel.SortReversed = true
	panel.selected[first] = 1
	panel.SearchBar.SetValue("not durable")
	panel.Renaming = true

	want := panel.SnapshotSession()
	restored := RestoreSession(want, root, false)

	assert.Equal(t, want, restored.SnapshotSession())
	assert.False(t, restored.IsFocused)
	assert.Empty(t, restored.selected)
	assert.Empty(t, restored.SearchBar.Value())
	assert.False(t, restored.Renaming)
}

func TestRestoreSessionPrunesInvalidTabsAndHistory(t *testing.T) {
	prepareSessionRestoreTest(t)
	root := t.TempDir()
	mixedCase := filepath.Join(root, "MixedCase")
	second := filepath.Join(root, "Second")
	file := filepath.Join(root, "file.txt")
	require.NoError(t, os.Mkdir(mixedCase, 0o755))
	require.NoError(t, os.Mkdir(second, 0o755))
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	missing := filepath.Join(root, "missing")

	restored := RestoreSession(session.PanelState{
		Tabs: []session.TabState{
			{Location: missing},
			{
				Location:       mixedCase,
				BackHistory:    []string{root, missing, file},
				ForwardHistory: []string{second, missing},
			},
			{Location: file},
			{Location: second},
		},
		ActiveTab: 3,
		SortKind:  int(sortmodel.SortByDate),
	}, root, true)

	want := session.PanelState{
		Tabs: []session.TabState{
			{
				Location:       mixedCase,
				BackHistory:    []string{root},
				ForwardHistory: []string{second},
			},
			{Location: second, BackHistory: []string{}, ForwardHistory: []string{}},
		},
		ActiveTab: 1,
		SortKind:  int(sortmodel.SortByDate),
	}
	assert.Equal(t, want, restored.SnapshotSession())
	assert.Equal(t, second, restored.Location)
	assert.True(t, restored.IsFocused)
}

func TestRestoreSessionClampsActiveTab(t *testing.T) {
	prepareSessionRestoreTest(t)
	root := t.TempDir()
	second := filepath.Join(root, "second")
	require.NoError(t, os.Mkdir(second, 0o755))
	tabs := []session.TabState{{Location: root}, {Location: second}}

	for _, test := range []struct {
		name string
		in   int
		want int
	}{
		{name: "negative", in: -7, want: 0},
		{name: "past end", in: 99, want: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			restored := RestoreSession(session.PanelState{Tabs: tabs, ActiveTab: test.in}, root, true)
			assert.Equal(t, test.want, restored.ActiveTabIndex())
		})
	}
}

func TestRestoreSessionFallsBackAndValidatesSort(t *testing.T) {
	prepareSessionRestoreTest(t)
	root := t.TempDir()
	restored := RestoreSession(session.PanelState{
		Tabs:         []session.TabState{{Location: filepath.Join(root, "missing")}},
		ActiveTab:    42,
		SortKind:     999,
		SortReversed: true,
	}, root, true)

	snapshot := restored.SnapshotSession()
	require.Len(t, snapshot.Tabs, 1)
	assert.Equal(t, root, snapshot.Tabs[0].Location)
	assert.Equal(t, 0, snapshot.ActiveTab)
	assert.Equal(t, int(sortmodel.SortByName), snapshot.SortKind)
	assert.True(t, snapshot.SortReversed)
}

func TestSessionSnapshotsAndRestoresUseIndependentCopies(t *testing.T) {
	prepareSessionRestoreTest(t)
	root := t.TempDir()
	child := filepath.Join(root, "child")
	require.NoError(t, os.Mkdir(child, 0o755))
	panel := newTabTestPanel(t, root)
	require.NoError(t, panel.UpdateCurrentFilePanelDir(child))

	snapshot := panel.SnapshotSession()
	snapshot.Tabs[0].BackHistory[0] = "changed"
	assert.Equal(t, root, panel.SnapshotSession().Tabs[0].BackHistory[0])

	input := panel.SnapshotSession()
	restored := RestoreSession(input, root, true)
	input.Tabs[0].Location = root
	input.Tabs[0].BackHistory[0] = "changed again"
	restoredSnapshot := restored.SnapshotSession()
	assert.Equal(t, child, restoredSnapshot.Tabs[0].Location)
	assert.Equal(t, root, restoredSnapshot.Tabs[0].BackHistory[0])
}
