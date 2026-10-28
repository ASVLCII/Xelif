//go:build xelif_demo

package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	variable "github.com/yorukot/superfile/src/config"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/pkg/utils"
)

type readmeDemoFrame struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	Duration int    `json:"duration"`
}

// Opt-in export of the real View renderer against disposable, explicitly named
// sample files. No running terminal, user folders, or background commands are used.
func TestExportReadmeDemo(t *testing.T) {
	output := os.Getenv("XELIF_DEMO_OUTPUT")
	if output == "" {
		t.Skip("set XELIF_DEMO_OUTPUT to a new directory to export README demo frames")
	}
	require.True(t, filepath.IsAbs(output), "demo output must be an absolute path")
	directories := []string{
		variable.SuperFileMainDir, variable.SuperFileDataDir, variable.SuperFileStateDir, variable.SuperFileCacheDir,
	}
	for _, directory := range directories {
		relative, err := filepath.Rel(output, directory)
		require.NoError(t, err)
		require.NotEqual(t, ".", relative)
		require.NotEqual(t, "..", relative)
		require.False(t, strings.HasPrefix(relative, ".."+string(filepath.Separator)))
		require.False(t, filepath.IsAbs(relative), "all demo storage must be inside the new output directory")
	}
	require.NoError(t, os.Mkdir(output, 0o700))
	for _, directory := range directories {
		require.NoError(t, os.MkdirAll(directory, 0o700))
	}
	root := filepath.Join(output, "Workspace")
	for _, name := range []string{"", "Design", "Documents", "Projects", "Releases"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, name), 0o700))
	}
	for _, name := range []string{
		"README.md", "app.go", "build.ps1", "config.toml", "design.svg",
		"interface.png", "notes.txt", "package.json", "release.zip", "theme.css", "walkthrough.mp4",
	} {
		path := filepath.Join(root, name)
		require.NoError(t, os.WriteFile(path, []byte(strings.Repeat("Xelif sample file.\n", 48)), 0o600))
		stamp := time.Date(2026, time.October, 22, 9, 30, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(path, stamp, stamp))
	}
	original := common.Config
	t.Cleanup(func() { common.SetConfig(original) })
	common.Config.SidebarWidth = 20
	common.Config.SidebarSections = []string{
		utils.SidebarSectionRecent, utils.SidebarSectionPinned, utils.SidebarSectionDisks,
	}
	common.Config.Nerdfont, common.Config.ColorFileIcons = true, true
	common.Config.FilePanelExtraColumns = 2
	common.Config.FilePanelNamePercent = 52
	m := defaultTestModel(root)
	defer m.fileModel.FilePreview.CleanUp()
	m.gitClient = nil
	m.firstUse = false
	for _, name := range []string{"Releases", "Documents", "Design"} {
		require.NoError(t, m.sidebarModel.AddRecent(filepath.Join(root, name)))
	}
	require.NoError(t, m.sidebarModel.TogglePinnedDirectory(root))
	m.sidebarModel.UpdateDirectories()
	TeaUpdate(m, tea.WindowSizeMsg{Width: 120, Height: 28})
	frames := []readmeDemoFrame{}
	capture := func(title string, duration int) {
		frames = append(frames, readmeDemoFrame{title, m.View().Content, duration})
	}
	capture("01 / Browse a workspace", 2400)
	m.openSortMenu()
	capture("Sort files / field and direction", 2400)
	m.sortModal.Close()
	panel := m.getFocusedFilePanel()
	for range 4 {
		panel.ListDown()
		capture("02 / Move one file at a time", 450)
	}
	panel.SearchBar.SetValue("design")
	panel.UpdateElementsIfNeeded(false, false)
	capture("03 / Filter the current folder", 2200)
	panel.SearchBar.SetValue("")
	panel.UpdateElementsIfNeeded(false, false)
	panel.SetCursorPosition(4)
	require.True(t, panel.SelectRangeTo(7))
	capture("04 / Select a range", 2000)
	panel.ResetSelected()
	m.openCommandPalette()
	capture("05 / Reach tools through Commands", 2600)
	m.commandPalette.Close()
	m.toggleFooter = true
	TeaUpdate(m, tea.WindowSizeMsg{Width: 120, Height: 28})
	capture("06 / Show information when you need it", 2400)
	data, err := json.MarshalIndent(frames, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(output, "frames.json"), data, 0o600))
}
