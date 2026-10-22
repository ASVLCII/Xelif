package filepanel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/rendering"
	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

func newTabTestPanel(t *testing.T, location string) Model {
	t.Helper()
	originalHotkeys := common.Hotkeys
	common.Hotkeys.SearchBar = []string{"/"}
	t.Cleanup(func() {
		common.Hotkeys = originalHotkeys //nolint:reassign // Restore the process-wide test configuration.
	})
	return New(location, true, "", sortmodel.SortByName, false)
}

func TestNewPanelStartsWithOneActiveTab(t *testing.T) {
	location := t.TempDir()
	panel := newTabTestPanel(t, location)

	assert.Equal(t, 1, panel.TabCount())
	assert.Equal(t, 0, panel.ActiveTabIndex())
	assert.Equal(t, location, panel.Location)
}

func TestNewTabActivatesRequestedDirectory(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	require.NoError(t, os.Mkdir(child, 0o755))
	panel := newTabTestPanel(t, root)

	err := panel.NewTab(child)

	require.NoError(t, err)
	assert.Equal(t, 2, panel.TabCount())
	assert.Equal(t, 1, panel.ActiveTabIndex())
	assert.Equal(t, child, panel.Location)
}

func TestCloseTabRejectsClosingLastTab(t *testing.T) {
	panel := newTabTestPanel(t, t.TempDir())

	err := panel.CloseTab()

	require.Error(t, err)
	assert.Equal(t, 1, panel.TabCount())
	assert.Equal(t, 0, panel.ActiveTabIndex())
}

func TestCloseTabReturnsToRemainingTab(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	require.NoError(t, os.Mkdir(child, 0o755))
	panel := newTabTestPanel(t, root)
	require.NoError(t, panel.NewTab(child))

	err := panel.CloseTab()

	require.NoError(t, err)
	assert.Equal(t, 1, panel.TabCount())
	assert.Equal(t, 0, panel.ActiveTabIndex())
	assert.Equal(t, root, panel.Location)
}

func TestNextTabCyclesThroughPanelTabs(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	require.NoError(t, os.Mkdir(child, 0o755))
	panel := newTabTestPanel(t, root)
	require.NoError(t, panel.NewTab(child))

	require.NoError(t, panel.NextTab())
	assert.Equal(t, 0, panel.ActiveTabIndex())
	assert.Equal(t, root, panel.Location)

	require.NoError(t, panel.NextTab())
	assert.Equal(t, 1, panel.ActiveTabIndex())
	assert.Equal(t, child, panel.Location)
}

func TestPreviousTabCyclesThroughPanelTabs(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	require.NoError(t, os.Mkdir(child, 0o755))
	panel := newTabTestPanel(t, root)
	require.NoError(t, panel.NewTab(child))

	require.NoError(t, panel.PreviousTab())
	assert.Equal(t, 0, panel.ActiveTabIndex())
	assert.Equal(t, root, panel.Location)

	require.NoError(t, panel.PreviousTab())
	assert.Equal(t, 1, panel.ActiveTabIndex())
	assert.Equal(t, child, panel.Location)
}

func TestBackReturnsToPreviousDirectory(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	grandchild := filepath.Join(child, "grandchild")
	require.NoError(t, os.MkdirAll(grandchild, 0o755))
	panel := newTabTestPanel(t, root)
	require.NoError(t, panel.UpdateCurrentFilePanelDir(child))
	require.NoError(t, panel.UpdateCurrentFilePanelDir(grandchild))

	err := panel.Back()

	require.NoError(t, err)
	assert.Equal(t, child, panel.Location)
}

func TestForwardReappliesBackedOutDirectory(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	grandchild := filepath.Join(child, "grandchild")
	require.NoError(t, os.MkdirAll(grandchild, 0o755))
	panel := newTabTestPanel(t, root)
	require.NoError(t, panel.UpdateCurrentFilePanelDir(child))
	require.NoError(t, panel.UpdateCurrentFilePanelDir(grandchild))
	require.NoError(t, panel.Back())

	err := panel.Forward()

	require.NoError(t, err)
	assert.Equal(t, grandchild, panel.Location)
}

func TestTabAtContentXMapsCompactTabLabels(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	require.NoError(t, os.Mkdir(child, 0o755))
	panel := newTabTestPanel(t, root)
	panel.SetWidth(80)
	require.NoError(t, panel.NewTab(child))

	index, ok := panel.TabAtContentX(1)
	assert.True(t, ok)
	assert.Equal(t, 0, index)

	secondStart := ansi.StringWidth(panel.tabLabel(0)) + 1
	index, ok = panel.TabAtContentX(secondStart)
	assert.True(t, ok)
	assert.Equal(t, 1, index)

	_, ok = panel.TabAtContentX(secondStart - 1)
	assert.False(t, ok)
	_, ok = panel.TabAtContentX(secondStart + ansi.StringWidth(panel.tabLabel(1)))
	assert.False(t, ok)
}

func TestRenderTopBarPlacesCompactTabsBeforeBreadcrumbs(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	require.NoError(t, os.Mkdir(child, 0o755))
	panel := newTabTestPanel(t, root)
	panel.width = 120
	require.NoError(t, panel.NewTab(child))
	originalIcon := common.FilePanelTopDirectoryIcon
	common.FilePanelTopDirectoryIcon = " D " //nolint:reassign // Test temporarily overrides a UI global.
	t.Cleanup(func() {
		common.FilePanelTopDirectoryIcon = originalIcon //nolint:reassign // Restore the UI global.
	})
	r := rendering.NewRendererWithAutoFixConfig(rendering.DefaultRendererConfig(3, 120))

	panel.renderTopBar(r)
	firstLine := strings.TrimRight(strings.Split(r.Render(), "\n")[0], " ")

	prefix := panel.tabLabel(0) + " " + panel.tabLabel(1) + " [+] [-]  D "
	assert.True(t, strings.HasPrefix(ansi.Strip(firstLine), prefix), "%q", ansi.Strip(firstLine))
}

func TestNewTabRejectsInvalidDirectoryWithoutChangingTabs(t *testing.T) {
	root := t.TempDir()
	panel := newTabTestPanel(t, root)

	err := panel.NewTab(filepath.Join(root, "missing"))

	require.Error(t, err)
	assert.Equal(t, 1, panel.TabCount())
	assert.Equal(t, 0, panel.ActiveTabIndex())
	assert.Equal(t, root, panel.Location)
}

func TestOrdinaryNavigationClearsForwardHistory(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	grandchild := filepath.Join(child, "grandchild")
	sibling := filepath.Join(root, "sibling")
	require.NoError(t, os.MkdirAll(grandchild, 0o755))
	require.NoError(t, os.Mkdir(sibling, 0o755))
	panel := newTabTestPanel(t, root)
	require.NoError(t, panel.UpdateCurrentFilePanelDir(child))
	require.NoError(t, panel.UpdateCurrentFilePanelDir(grandchild))
	require.NoError(t, panel.Back())
	require.NoError(t, panel.UpdateCurrentFilePanelDir(sibling))

	require.NoError(t, panel.Forward())
	assert.Equal(t, sibling, panel.Location)
}

func TestNavigationHistoryIsIsolatedPerTab(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")
	require.NoError(t, os.Mkdir(first, 0o755))
	require.NoError(t, os.Mkdir(second, 0o755))
	panel := newTabTestPanel(t, root)
	require.NoError(t, panel.UpdateCurrentFilePanelDir(first))
	require.NoError(t, panel.NewTab(root))
	require.NoError(t, panel.UpdateCurrentFilePanelDir(second))
	require.NoError(t, panel.Back())
	require.NoError(t, panel.PreviousTab())
	require.NoError(t, panel.Back())

	require.NoError(t, panel.NextTab())
	require.NoError(t, panel.Forward())
	assert.Equal(t, second, panel.Location)

	require.NoError(t, panel.PreviousTab())
	require.NoError(t, panel.Forward())
	assert.Equal(t, first, panel.Location)
}

func TestBackRestoresCachedCursorAndRenderIndex(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	require.NoError(t, os.Mkdir(child, 0o755))
	panel := newTabTestPanel(t, root)
	panel.cursor = 7
	panel.renderIndex = 3
	require.NoError(t, panel.UpdateCurrentFilePanelDir(child))
	panel.cursor = 2
	panel.renderIndex = 1

	require.NoError(t, panel.Back())

	assert.Equal(t, 7, panel.GetCursor())
	assert.Equal(t, 3, panel.GetRenderIndex())
}

func TestBreadcrumbHitTestingAccountsForTabPrefix(t *testing.T) {
	root := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	location := filepath.Join(root, "alpha")
	panel := Model{
		Location:  location,
		width:     80,
		tabs:      []panelTab{{location: location}},
		activeTab: 0,
	}
	originalIcon := common.FilePanelTopDirectoryIcon
	common.FilePanelTopDirectoryIcon = " D " //nolint:reassign // Test temporarily overrides a UI global.
	t.Cleanup(func() {
		common.FilePanelTopDirectoryIcon = originalIcon //nolint:reassign // Restore the UI global.
	})

	path, ok := panel.BreadcrumbPathAtContentX(
		panel.tabBarWidth() + ansi.StringWidth(common.FilePanelTopDirectoryIcon),
	)

	require.True(t, ok)
	assert.Equal(t, root, path)
	_, ok = panel.BreadcrumbPathAtContentX(1)
	assert.False(t, ok)
}

func TestRenderedTabBarDistinguishesActiveTab(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	require.NoError(t, os.Mkdir(child, 0o755))
	panel := newTabTestPanel(t, root)
	panel.SetWidth(80)
	require.NoError(t, panel.NewTab(child))
	originalActiveStyle := common.FilePanelCursorStyle
	originalInactiveStyle := common.FilePanelTopPathStyle
	common.FilePanelCursorStyle = lipgloss.NewStyle().Bold(true) //nolint:reassign // Test render styling.
	common.FilePanelTopPathStyle = lipgloss.NewStyle()           //nolint:reassign // Test render styling.
	t.Cleanup(func() {
		common.FilePanelCursorStyle = originalActiveStyle    //nolint:reassign // Restore the UI global.
		common.FilePanelTopPathStyle = originalInactiveStyle //nolint:reassign // Restore the UI global.
	})

	secondActive := panel.renderTabBar()
	require.NoError(t, panel.PreviousTab())
	firstActive := panel.renderTabBar()

	expected := panel.tabLabel(0) + " " + panel.tabLabel(1) + " [+] [-] "
	assert.Equal(t, expected, ansi.Strip(firstActive))
	assert.Equal(t, expected, ansi.Strip(secondActive))
	assert.NotEqual(t, firstActive, secondActive)
}

func TestActivateTabSelectsRequestedTab(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	require.NoError(t, os.Mkdir(child, 0o755))
	panel := newTabTestPanel(t, root)
	require.NoError(t, panel.NewTab(child))

	require.NoError(t, panel.ActivateTab(0))
	assert.Equal(t, 0, panel.ActiveTabIndex())
	assert.Equal(t, root, panel.Location)
	require.Error(t, panel.ActivateTab(99))
}
