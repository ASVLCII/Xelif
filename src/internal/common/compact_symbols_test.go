package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/config/icon"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestNerdFileIconsRemainRecognizableAndMonochrome(t *testing.T) {
	old := Config
	t.Cleanup(func() { SetConfig(old) })
	Config.Theme = "monochrome"
	Config.ColorFileIcons = false
	Config.PlainMode = false
	oldFolder := icon.Folders["folder"]
	t.Cleanup(func() { icon.Folders["folder"] = oldFolder })
	icon.InitIcon(true, "")
	for _, tc := range []struct {
		name string
		dir  bool
		want string
	}{
		{"folder", true, icon.Folders["folder"].Icon},
		{"photo.png", false, icon.Icons["image"].Icon},
		{"notes.txt", false, icon.Icons["txt"].Icon},
		{"unknown.bin", false, icon.Icons["file"].Icon},
	} {
		style := GetElementIcon(tc.name, tc.dir, false, true)
		require.NotEmpty(t, style.Icon)
		require.Equal(t, tc.want, style.Icon)
		require.Equal(t, Theme.FilePanelFG, style.Color)
	}
}

func TestCompactFileMarkersAndPlainFallback(t *testing.T) {
	old := Config
	t.Cleanup(func() { SetConfig(old) })
	Config.PlainMode = false
	for _, tc := range []struct {
		name            string
		directory, link bool
		marker          string
	}{
		{"folder", true, false, "▸"}, {"photo.PNG", false, false, "▧"},
		{"photo.JPEG", false, false, "▧"}, {"archive.tar.gz", false, false, "▥"},
		{"report.pdf", false, false, "≡"}, {"notes.txt", false, false, "≡"},
		{"readme.md", false, false, "≡"}, {"other.bin", false, false, "□"},
		{"photo.png", false, true, "↗"}, {"folder", true, true, "↗"},
	} {
		t.Run(tc.name+tc.marker, func(t *testing.T) {
			style := GetElementIcon(tc.name, tc.directory, tc.link, false)
			require.Equal(t, tc.marker, style.Icon)
			require.Equal(t, 1, ansi.StringWidth(style.Icon), "symbols must occupy one terminal column")
		})
	}
	Config.PlainMode = true
	require.Empty(t, GetElementIcon("photo.png", false, false, false).Icon)
}

func TestCompactDefaultsPreserveUserPreferences(t *testing.T) {
	defaults, err := os.ReadFile(filepath.Join("..", "..", "superfile_config", "config.toml"))
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "config.toml")
	user := []byte("theme = 'custom'\nnerdfont = true\nshow_image_preview = false\nshow_image_thumbnails = true\n")
	require.NoError(t, os.WriteFile(path, user, 0600))
	var cfg ConfigType
	require.Error(t, utils.LoadTomlFile(path, string(defaults), &cfg, false, false))
	require.True(t, cfg.Nerdfont)
	require.Equal(t, "custom", cfg.Theme)
	require.False(t, cfg.ShowImagePreview)
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, user, after)
}
