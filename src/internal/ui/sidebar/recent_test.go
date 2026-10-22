package sidebar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestRecentManagerMaintainsNewestFirstUniqueAndCapped(t *testing.T) {
	testDir := t.TempDir()
	recentFile := filepath.Join(testDir, "recent.json")
	mgr := NewRecentFileManager(recentFile)

	paths := make([]string, 12)
	for i := range paths {
		paths[i] = filepath.Join(testDir, fmt.Sprintf("dir-%02d", i))
		utils.SetupDirectories(t, paths[i])
		require.NoError(t, mgr.Add(paths[i]))
	}

	recent := mgr.Load()
	require.Len(t, recent, maxRecentDirectories)
	for i, dir := range recent {
		assert.Equal(t, paths[len(paths)-1-i], dir.Location)
	}

	// Re-adding an older path moves it to the front without creating a duplicate.
	require.NoError(t, mgr.Add(paths[5]))
	reloadedMgr := NewRecentFileManager(recentFile)
	recent = reloadedMgr.Load()
	require.Len(t, recent, maxRecentDirectories)
	assert.Equal(t, paths[5], recent[0].Location)

	seen := make(map[string]bool, len(recent))
	for _, dir := range recent {
		assert.False(t, seen[dir.Location], "recent path should occur only once: %s", dir.Location)
		seen[dir.Location] = true
	}
}

func TestRecentManagerRemovesMissingAndNonDirectoryPaths(t *testing.T) {
	testDir := t.TempDir()
	existingDir := filepath.Join(testDir, "existing")
	missingDir := filepath.Join(testDir, "missing")
	filePath := filepath.Join(testDir, "file.txt")
	recentFile := filepath.Join(testDir, "recent.json")

	utils.SetupDirectories(t, existingDir)
	utils.SetupFiles(t, filePath)

	stored := []directory{
		{Location: missingDir, Name: "missing"},
		{Location: existingDir, Name: "existing"},
		{Location: filePath, Name: "file.txt"},
		{Location: existingDir, Name: "duplicate"},
	}
	data, err := json.Marshal(stored)
	require.NoError(t, err)
	utils.SetupFilesWithData(t, data, recentFile)

	mgr := RecentManager{filePath: recentFile}
	recent := mgr.Load()
	require.Equal(t, []directory{{Location: existingDir, Name: "existing"}}, recent)

	persistedData, err := os.ReadFile(recentFile)
	require.NoError(t, err)
	var persisted []directory
	require.NoError(t, json.Unmarshal(persistedData, &persisted))
	assert.Equal(t, recent, persisted)
}

func TestModelAddRecentPersistsPath(t *testing.T) {
	testDir := t.TempDir()
	recentDir := filepath.Join(testDir, "project")
	utils.SetupDirectories(t, recentDir)

	mgr := NewRecentFileManager(filepath.Join(testDir, "recent.json"))
	sidebar := Model{recentMgr: &mgr}

	require.NoError(t, sidebar.AddRecent(recentDir))
	require.Equal(t, []directory{{Location: recentDir, Name: "project"}}, mgr.Load())
}

func TestRecentAndFavoritesSectionsRender(t *testing.T) {
	recentDir := directory{Name: "recent-project", Location: filepath.Join(t.TempDir(), "recent")}
	favoriteDir := directory{Name: "favorite-project", Location: filepath.Join(t.TempDir(), "favorite")}
	directories := formDirectorySlice(
		[]directory{{Name: "home", Location: filepath.Join(t.TempDir(), "home")}},
		[]directory{recentDir},
		[]directory{favoriteDir},
		nil,
		[]string{utils.SidebarSectionHome, utils.SidebarSectionRecent, utils.SidebarSectionPinned},
	)

	require.Len(t, directories, 5)
	assert.Equal(t, recentDividerDir, directories[1])
	assert.Equal(t, utils.SidebarSectionRecent, directories[2].Section)
	assert.Equal(t, pinnedDividerDir, directories[3])
	assert.Equal(t, utils.SidebarSectionPinned, directories[4].Section)

	originalConfig := common.Config
	config := common.Config
	config.BorderTop = "-"
	config.BorderBottom = "-"
	config.BorderLeft = "|"
	config.BorderRight = "|"
	config.BorderTopLeft = "+"
	config.BorderTopRight = "+"
	config.BorderBottomLeft = "+"
	config.BorderBottomRight = "+"
	config.BorderMiddleLeft = "+"
	config.BorderMiddleRight = "+"
	common.SetConfig(config)
	t.Cleanup(func() { common.SetConfig(originalConfig) })

	originalTitle := common.SideBarSuperfileTitle
	originalRecent := common.SideBarRecentDivider
	originalPinned := common.SideBarPinnedDivider
	originalNone := common.SideBarNoneText
	originalHotkeys := common.Hotkeys
	common.Hotkeys.ConfirmTyping = []string{"enter"}
	common.Hotkeys.Quit = []string{"q"}
	common.LoadPrerenderedVariables()
	t.Cleanup(func() {
		common.SideBarSuperfileTitle = originalTitle //nolint:reassign // Restore process-wide UI fixtures after this test.
		common.SideBarRecentDivider = originalRecent //nolint:reassign // Restore process-wide UI fixtures after this test.
		common.SideBarPinnedDivider = originalPinned //nolint:reassign // Restore process-wide UI fixtures after this test.
		common.SideBarNoneText = originalNone        //nolint:reassign // Restore process-wide UI fixtures after this test.
		common.Hotkeys = originalHotkeys             //nolint:reassign // Restore process-wide UI fixtures after this test.
	})

	sidebar := Model{directories: directories, width: 40, height: 20}
	rendered := ansi.Strip(sidebar.Render(false, ""))
	assert.Contains(t, rendered, "Recent")
	assert.Contains(t, rendered, "Favorites")
	assert.NotContains(t, rendered, "Pinned")
}

func TestUpdateDirectoriesKeepsFocusedRecentAfterReorder(t *testing.T) {
	testDir := t.TempDir()
	first := filepath.Join(testDir, "first")
	second := filepath.Join(testDir, "second")
	utils.SetupDirectories(t, first, second)

	recentMgr := NewRecentFileManager(filepath.Join(testDir, "recent.json"))
	require.NoError(t, recentMgr.Save([]directory{
		{Location: first, Name: "first"},
		{Location: second, Name: "second"},
	}))
	pinnedMgr := NewPinnedFileManager(filepath.Join(testDir, "pinned.json"))
	sidebar := Model{
		directories: getDirectories(&pinnedMgr, &recentMgr, []string{utils.SidebarSectionRecent}),
		cursor:      1,
		pinnedMgr:   &pinnedMgr,
		recentMgr:   &recentMgr,
		sections:    []string{utils.SidebarSectionRecent},
		height:      20,
	}
	require.Equal(t, second, sidebar.GetCurrentDirectoryLocation())
	require.NoError(t, recentMgr.Add(second))

	sidebar.UpdateDirectories()

	assert.Equal(t, second, sidebar.GetCurrentDirectoryLocation())
}
