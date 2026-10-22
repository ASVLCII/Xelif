package filemodel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/session"
)

func prepareFileModelSessionTest(t *testing.T) {
	t.Helper()
	originalHotkeys := common.Hotkeys
	common.Hotkeys.SearchBar = []string{"/"}
	t.Cleanup(func() {
		common.Hotkeys = originalHotkeys //nolint:reassign // Restore the process-wide test configuration.
	})
}

func TestSessionSnapshotIncludesPanelsFocusAndDotFiles(t *testing.T) {
	prepareFileModelSessionTest(t)
	root := t.TempDir()
	second := filepath.Join(root, "second")
	require.NoError(t, os.Mkdir(second, 0o755))
	m := New([]string{root, second}, true)
	require.True(t, m.FocusPanel(1))

	panels, focused, showDotFiles := m.SnapshotSession()

	require.Len(t, panels, 2)
	assert.Equal(t, root, panels[0].Tabs[0].Location)
	assert.Equal(t, second, panels[1].Tabs[0].Location)
	assert.Equal(t, 1, focused)
	assert.True(t, showDotFiles)
}

func TestRestoreSessionPrunesInvalidPanelsAndPreservesFocusIdentity(t *testing.T) {
	prepareFileModelSessionTest(t)
	root := t.TempDir()
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")
	for _, location := range []string{first, second} {
		require.NoError(t, os.Mkdir(location, 0o755))
	}
	m := New([]string{root}, false)
	m.MaxFilePanel = 4
	states := []session.PanelState{
		{Tabs: []session.TabState{{Location: first}}},
		{Tabs: []session.TabState{{Location: filepath.Join(root, "missing")}}},
		{Tabs: []session.TabState{{Location: second}}},
	}

	m.RestoreSession(states, 2, true, root)

	require.Len(t, m.FilePanels, 2)
	assert.Equal(t, first, m.FilePanels[0].Location)
	assert.Equal(t, second, m.FilePanels[1].Location)
	assert.Equal(t, 1, m.FocusedPanelIndex)
	assert.False(t, m.FilePanels[0].IsFocused)
	assert.True(t, m.FilePanels[1].IsFocused)
	assert.True(t, m.DisplayDotFiles)
}

func TestRestoreSessionRetainsFallbackWhenNoPanelIsValid(t *testing.T) {
	prepareFileModelSessionTest(t)
	root := t.TempDir()
	m := New([]string{root}, false)
	m.MaxFilePanel = 3

	m.RestoreSession([]session.PanelState{{
		Tabs: []session.TabState{{Location: filepath.Join(root, "missing")}},
	}}, 99, true, root)

	require.Len(t, m.FilePanels, 1)
	assert.Equal(t, root, m.FilePanels[0].Location)
	assert.Equal(t, 0, m.FocusedPanelIndex)
	assert.True(t, m.FilePanels[0].IsFocused)
}

func TestRestoreSessionObeysMaxFilePanelAndClampsFocus(t *testing.T) {
	prepareFileModelSessionTest(t)
	root := t.TempDir()
	locations := make([]string, 4)
	states := make([]session.PanelState, 4)
	for index := range locations {
		locations[index] = filepath.Join(root, string(rune('a'+index)))
		require.NoError(t, os.Mkdir(locations[index], 0o755))
		states[index] = session.PanelState{Tabs: []session.TabState{{Location: locations[index]}}}
	}
	m := New([]string{root}, false)
	m.MaxFilePanel = 2

	m.RestoreSession(states, 3, false, root)

	require.Len(t, m.FilePanels, 2)
	assert.Equal(t, locations[0], m.FilePanels[0].Location)
	assert.Equal(t, locations[1], m.FilePanels[1].Location)
	assert.Equal(t, 1, m.FocusedPanelIndex)
	assert.False(t, m.FilePanels[0].IsFocused)
	assert.True(t, m.FilePanels[1].IsFocused)
}

func TestRestoreSessionClampsNegativeFocus(t *testing.T) {
	prepareFileModelSessionTest(t)
	root := t.TempDir()
	second := filepath.Join(root, "second")
	require.NoError(t, os.Mkdir(second, 0o755))
	m := New([]string{root}, false)
	m.MaxFilePanel = 2
	m.RestoreSession([]session.PanelState{
		{Tabs: []session.TabState{{Location: root}}},
		{Tabs: []session.TabState{{Location: second}}},
	}, -5, false, root)

	assert.Equal(t, 0, m.FocusedPanelIndex)
	assert.True(t, m.FilePanels[0].IsFocused)
	assert.False(t, m.FilePanels[1].IsFocused)
}

func TestSessionSnapshotsAndRestoreInputsAreIndependent(t *testing.T) {
	prepareFileModelSessionTest(t)
	root := t.TempDir()
	child := filepath.Join(root, "child")
	require.NoError(t, os.Mkdir(child, 0o755))
	m := New([]string{root}, false)
	m.MaxFilePanel = 2
	states := []session.PanelState{{Tabs: []session.TabState{{
		Location:    child,
		BackHistory: []string{root},
	}}}}

	m.RestoreSession(states, 0, false, root)
	states[0].Tabs[0].Location = root
	states[0].Tabs[0].BackHistory[0] = "changed"
	panels, _, _ := m.SnapshotSession()
	assert.Equal(t, child, panels[0].Tabs[0].Location)
	assert.Equal(t, root, panels[0].Tabs[0].BackHistory[0])

	panels[0].Tabs[0].Location = root
	panels[0].Tabs[0].BackHistory[0] = "changed again"
	again, _, _ := m.SnapshotSession()
	assert.Equal(t, child, again[0].Tabs[0].Location)
	assert.Equal(t, root, again[0].Tabs[0].BackHistory[0])
}
