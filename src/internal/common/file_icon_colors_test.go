package common

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileIconCategoriesAndContrast(t *testing.T) {
	oldConfig, oldTheme := Config, Theme
	t.Cleanup(func() { Config, Theme = oldConfig, oldTheme })
	Config.ColorFileIcons, Config.PlainMode, Config.TransparentBackground = true, false, false
	for _, tc := range []struct {
		file      string
		dir, link bool
		category  int
	}{
		{"folder", true, false, 0}, {"photo.png", false, true, 0},
		{"PHOTO.PNG", false, false, 1}, {"picture.webp", false, false, 1},
		{"README.md", false, false, 2}, {"README", false, false, 2},
		{"notes.txt", false, false, 2}, {"report.docx", false, false, 2},
		{"table.csv", false, false, 2}, {"archive.tar.gz", false, false, 3},
		{"source.go", false, false, 4}, {"go.mod", false, false, 4},
		{"settings.yaml", false, false, 4}, {"launch.ps1", false, false, 4},
		{"song.mp3", false, false, 5}, {"movie.mp4", false, false, 5},
		{"unknown.xyz", false, false, 6}, {"data.bin", false, false, 6},
	} {
		require.Equal(t, tc.category, fileIconCategory(tc.file, tc.dir, tc.link), tc.file)
		for _, bg := range []string{"#181818", "#444444", "#ffffff", "#eeeeee"} {
			Theme.FilePanelBG, Theme.FilePanelFG = bg, "#dddddd"
			color := fileTypeIconColor(tc.file, tc.dir, tc.link)
			luminance, ok := iconColorLuminance(bg)
			require.True(t, ok)
			require.GreaterOrEqual(t, iconColorContrast(color, luminance), 4.5, tc.file+bg)
		}
	}
}

func TestFileIconColorsRespectOptOutAndUnknownBackground(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	require.NoError(t, os.Unsetenv("NO_COLOR"))
	oldConfig, oldTheme := Config, Theme
	t.Cleanup(func() { Config, Theme = oldConfig, oldTheme })
	Theme.FilePanelBG, Theme.FilePanelFG = "#181818", "#dddddd"
	Config.ColorFileIcons = true
	Config.PlainMode, Config.TransparentBackground = false, false
	colored := GetElementIcon("photo.png", false, false, false)
	require.Equal(t, "#be95ff", colored.Color)
	Config.ColorFileIcons = false
	require.Equal(t, Theme.FilePanelFG, GetElementIcon("photo.png", false, false, false).Color)
	Config.ColorFileIcons, Config.PlainMode = true, true
	require.Equal(t, Theme.FilePanelFG, GetElementIcon("photo.png", false, false, false).Color)
	Config.PlainMode = false
	Config.TransparentBackground = true
	require.Equal(t, Theme.FilePanelFG, fileTypeIconColor("photo.png", false, false))
	Config.TransparentBackground = false
	Theme.FilePanelBG = "terminal-default"
	require.Equal(t, Theme.FilePanelFG, fileTypeIconColor("photo.png", false, false))
	t.Setenv("NO_COLOR", "1")
	require.Empty(t, GetElementIcon("photo.png", false, false, false).Color)
}

func TestIconColorLuminance(t *testing.T) {
	black, ok := iconColorLuminance("#000000")
	require.True(t, ok)
	require.Zero(t, black)
	white, ok := iconColorLuminance("#ffffff")
	require.True(t, ok)
	require.InDelta(t, 1, white, 1e-12)
	require.InDelta(t, 21, iconColorContrast("#ffffff", black), 1e-12)
	for _, invalid := range []string{"", "#fff", "#gggggg", "1234567"} {
		_, ok := iconColorLuminance(invalid)
		require.False(t, ok)
	}
}
