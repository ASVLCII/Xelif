package common

import (
	"testing"

	"charm.land/lipgloss/v2"
)

// These benchmarks keep the same palette and names across baseline/final runs.
func BenchmarkFileIconColor(b *testing.B) {
	oldConfig := Config
	b.Cleanup(func() { Config = oldConfig })
	Config.TransparentBackground = false
	names := [...]string{"folder", "PHOTO.PNG", "README.md", "archive.zip", "source.go", "movie.mp4", "unknown.xyz"}
	b.ReportAllocs()
	for i := range b.N {
		_ = fileTypeIconColorOnBackground(names[i%len(names)], i%len(names) == 0, false, "#181818", "#dddddd")
	}
}

func BenchmarkFileNameRender(b *testing.B) {
	oldConfig, oldTheme := Config, Theme
	b.Cleanup(func() { Config, Theme = oldConfig, oldTheme })
	Config.ColorFileIcons, Config.Nerdfont = true, true
	Config.PlainMode, Config.TransparentBackground = false, false
	Theme.FilePanelBG, Theme.FilePanelFG = "#181818", "#dddddd"
	b.ReportAllocs()
	for range b.N {
		_ = FilePanelItemRenderWithIcon("Example-report-2026.md", 70, false, false, false, lipgloss.Color("#181818"))
	}
}
