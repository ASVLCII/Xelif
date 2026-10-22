package common

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// Independent pre-optimization calculation: keep color selection and contrast
// thresholds byte-for-byte equivalent when the theme changes at runtime.
func referenceFileIconColor(file string, directory, link bool, background, fallback string) string {
	bg, ok := iconColorLuminance(background)
	if !ok || Config.TransparentBackground {
		return fallback
	}
	palette := [...]struct{ dark, light string }{
		{"#78a9ff", "#0043ce"}, {"#be95ff", "#6929c4"},
		{"#6fdc8c", "#0e6027"}, {"#f1c21b", "#684e00"},
		{"#3ddbd9", "#005d5d"}, {"#ff7eb6", "#9f1853"},
		{"#c6c6c6", "#525252"},
	}
	p := palette[fileIconCategory(file, directory, link)]
	best := p.dark
	if referenceIconContrast(p.light, bg) > referenceIconContrast(best, bg) {
		best = p.light
	}
	if referenceIconContrast(best, bg) < 4.5 {
		return fallback
	}
	return best
}

func referenceIconContrast(hex string, background float64) float64 {
	foreground, _ := iconColorLuminance(hex)
	return (math.Max(foreground, background) + .05) / (math.Min(foreground, background) + .05)
}

func TestFileIconPaletteMatchesReference(t *testing.T) {
	oldConfig := Config
	t.Cleanup(func() { Config = oldConfig })
	names := []string{"PHOTO.PNG", "README.md", "archive.tar.gz", "source.go", "movie.mp4", "unknown.xyz"}
	backgrounds := []string{"", "terminal-default", "#gggggg", "#fff", "#ffffff", "#181818", "#A7BCDE", "#143b29"}
	for gray := range 256 {
		backgrounds = append(backgrounds, fmt.Sprintf("#%02x%02x%02x", gray, gray, gray))
	}
	for _, transparent := range []bool{false, true} {
		Config.TransparentBackground = transparent
		for _, background := range backgrounds {
			for _, name := range names {
				for _, directoryAndLink := range [][2]bool{{false, false}, {true, false}, {false, true}} {
					directory, link := directoryAndLink[0], directoryAndLink[1]
					require.Equal(
						t,
						referenceFileIconColor(name, directory, link, background, "fallback"),
						fileTypeIconColorOnBackground(name, directory, link, background, "fallback"),
						"%s background=%s directory=%t link=%t transparent=%t",
						name,
						background,
						directory,
						link,
						transparent,
					)
				}
			}
		}
	}
}
