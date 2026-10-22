package common

import (
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yorukot/superfile/src/config/icon"
)

const (
	iconNavigation = iota
	iconImage
	iconDocument
	iconArchive
	iconTechnical
	iconMedia
	iconOther
)

const iconContrastTarget = 4.5

type iconPaletteColor struct {
	hex       string
	luminance float64
}

func newIconPaletteColor(hex string) iconPaletteColor {
	luminance, _ := iconColorLuminance(hex)
	return iconPaletteColor{hex: hex, luminance: luminance}
}

// Only the fixed palette is precomputed. Theme backgrounds are still evaluated
// on every call, so theme changes and transparency never use stale colors.
//
//nolint:gochecknoglobals // Read-only categorical palette; precompute fixed sRGB conversions once.
var fileIconPalette = [...]struct{ dark, light iconPaletteColor }{
	{newIconPaletteColor("#78a9ff"), newIconPaletteColor("#0043ce")}, // navigation: blue
	{newIconPaletteColor("#be95ff"), newIconPaletteColor("#6929c4")}, // images: purple
	{newIconPaletteColor("#6fdc8c"), newIconPaletteColor("#0e6027")}, // documents: green
	{newIconPaletteColor("#f1c21b"), newIconPaletteColor("#684e00")}, // archives: amber
	{newIconPaletteColor("#3ddbd9"), newIconPaletteColor("#005d5d")}, // technical: cyan
	{newIconPaletteColor("#ff7eb6"), newIconPaletteColor("#9f1853")}, // media: pink
	{newIconPaletteColor("#c6c6c6"), newIconPaletteColor("#525252")}, // unknown: neutral
}

func fileIconCategory(file string, isDir, isLink bool) int {
	if isLink || isDir {
		return iconNavigation
	}
	name := strings.ToLower(filepath.Base(file))
	key := strings.TrimPrefix(filepath.Ext(name), ".")
	if alias, ok := icon.Aliases[key]; ok {
		key = alias
	}
	if alias, ok := icon.Aliases[name]; ok {
		key = alias
	} else if _, ok := icon.Icons[name]; ok {
		key = name
	}
	switch key {
	case "image", "psd", "ai", "avif", "heic", "heif":
		return iconImage
	case "txt", "md", "pdf", "doc", "rtf", "odt", "ebook", "xls", "ppt", "log", "rst", "tex", "epub", "ods", "odp":
		return iconDocument
	case "zip", "iso":
		return iconArchive
	case "audio", "video", "opus":
		return iconMedia
	case "file", "", "bin":
		return iconOther
	default:
		if _, known := icon.Icons[key]; known || key == "ps1" || key == "sql" || key == "conf" || key == "cfg" {
			return iconTechnical
		}
		return iconOther
	}
}

func fileTypeIconColor(file string, isDir, isLink bool) string {
	return fileTypeIconColorOnBackground(file, isDir, isLink, Theme.FilePanelBG, Theme.FilePanelFG)
}

// SidebarIconColor shares the file palette but checks the sidebar's background.
func SidebarIconColor(glyph string, selected bool) string {
	bg, fg := Theme.SidebarBG, Theme.SidebarFG
	if selected {
		bg, fg = Theme.SidebarItemSelectedBG, Theme.SidebarItemSelectedFG
	}
	if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
		return ""
	}
	if !Config.ColorFileIcons || Config.PlainMode || glyph == "" {
		return fg
	}
	file, directory := "folder", true
	switch glyph {
	case icon.Documents, icon.Templates:
		file, directory = "document.txt", false
	case icon.Pictures:
		file, directory = "image.png", false
	case icon.Videos, icon.Music:
		file, directory = "media.mp4", false
	case icon.Disk:
		file, directory = "unknown.bin", false
	}
	return fileTypeIconColorOnBackground(file, directory, false, bg, fg)
}

func fileTypeIconColorOnBackground(file string, isDir, isLink bool, background, fallback string) string {
	bg, ok := iconColorLuminance(background)
	if !ok || Config.TransparentBackground {
		// The actual terminal background is unknown; do not promise contrast.
		return fallback
	}
	// One category palette, not arbitrary per-extension colors. IBM Carbon hues.
	p := fileIconPalette[fileIconCategory(file, isDir, isLink)]
	best := p.dark
	if iconLuminanceContrast(p.light.luminance, bg) > iconLuminanceContrast(best.luminance, bg) {
		best = p.light
	}
	if iconLuminanceContrast(best.luminance, bg) < iconContrastTarget {
		return fallback
	}
	return best.hex
}

// WCAG relative luminance for explicit sRGB theme colors.
func iconColorLuminance(hex string) (float64, bool) {
	const (
		byteMax         = 255
		redShift        = 16
		greenShift      = 8
		srgbThreshold   = 0.04045
		srgbLinearScale = 12.92
		srgbOffset      = 0.055
		srgbScale       = 1.055
		srgbExponent    = 2.4
	)
	if len(hex) != 7 || hex[0] != '#' {
		return 0, false
	}
	n, err := strconv.ParseUint(hex[1:], 16, 24)
	if err != nil {
		return 0, false
	}
	linear := func(c uint64) float64 {
		v := float64(c) / byteMax
		if v <= srgbThreshold {
			return v / srgbLinearScale
		}
		return math.Pow((v+srgbOffset)/srgbScale, srgbExponent)
	}
	return .2126*linear(n>>redShift) + .7152*linear((n>>greenShift)&byteMax) + .0722*linear(n&byteMax), true
}

func iconColorContrast(foreground string, backgroundLuminance float64) float64 {
	fg, ok := iconColorLuminance(foreground)
	if !ok {
		return 0
	}
	return iconLuminanceContrast(fg, backgroundLuminance)
}

func iconLuminanceContrast(foreground, background float64) float64 {
	const luminanceOffset = .05
	return (math.Max(foreground, background) + luminanceOffset) / (math.Min(foreground, background) + luminanceOffset)
}
