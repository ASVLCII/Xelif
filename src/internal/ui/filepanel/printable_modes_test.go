package filepanel

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/yorukot/superfile/src/config/icon"
	"github.com/yorukot/superfile/src/internal/common"
)

// Compile this same fixture against baseline and candidate production sources
// and compare PRINTABLE_MATRIX. The digest includes the exact ANSI bytes, case
// labels and frame lengths; it does not normalize styling or terminal width.
func TestFilePanelPrintableModesFingerprint(t *testing.T) {
	// InitIcon(false) changes package-level symbols as well as folder styling.
	// Restore every affected symbol, not only the folder entry saved by the
	// existing render fixture. This test must not run in parallel.
	restoreIcons := snapshotPrintableModeIcons()
	t.Cleanup(restoreIcons)
	prepareFilePanelRenderBenchmark(t)
	resetNerdIcons := snapshotPrintableModeIcons()
	baseConfig, baseTheme := common.Config, common.Theme
	var allFrames strings.Builder
	totalFrames := 0
	for _, mode := range []printableRenderMode{
		{name: "nerd-color", nerd: true, colorIcons: true},
		{name: "unicode-color", colorIcons: true},
		{name: "plain", plain: true, colorIcons: true},
		{name: "no-color", nerd: true, colorIcons: true, noColor: true},
		{name: "custom-light", nerd: true, colorIcons: true, light: true},
		{name: "neutral-icons", nerd: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			resetNerdIcons()
			t.Cleanup(resetNerdIcons)
			configurePrintableRenderMode(t, mode, baseConfig, baseTheme)
			frames, count := printableRenderFrames(t)
			fmt.Fprintf(&allFrames, "%s:%d:%s", mode.name, len(frames), frames)
			totalFrames += count
			t.Logf("PRINTABLE_MODE mode=%s frames=%d sha256=%x", mode.name, count, sha256.Sum256([]byte(frames)))
		})
	}
	t.Logf("PRINTABLE_MATRIX frames=%d sha256=%x", totalFrames, sha256.Sum256([]byte(allFrames.String())))
}

type printableRenderMode struct {
	name       string
	nerd       bool
	colorIcons bool
	plain      bool
	noColor    bool
	light      bool
}

func configurePrintableRenderMode(
	t *testing.T,
	mode printableRenderMode,
	config common.ConfigType,
	theme common.ThemeType,
) {
	t.Helper()
	config.Nerdfont, config.ColorFileIcons, config.PlainMode = mode.nerd, mode.colorIcons, mode.plain
	config.ShowSelectIcons, config.ShowPanelFooterInfo = true, true
	if mode.plain {
		// Mirror the public plain-mode startup settings without importing the
		// root internal package (which itself imports filepanel).
		config.Nerdfont, config.ShowSelectIcons, config.ShowImagePreview = false, false, false
		config.BorderTop, config.BorderBottom = "-", "-"
		config.BorderLeft, config.BorderRight = "|", "|"
		config.BorderTopLeft, config.BorderTopRight = "+", "+"
		config.BorderBottomLeft, config.BorderBottomRight = "+", "+"
		config.BorderMiddleLeft, config.BorderMiddleRight = "+", "+"
	}
	if mode.noColor {
		t.Setenv("NO_COLOR", "1")
	}
	if mode.light {
		theme.FilePanelBG, theme.FilePanelFG = "#f8f8f6", "#202426"
		theme.FilePanelItemSelectedBG, theme.FilePanelItemSelectedFG = "#dce2e7", "#161a1e"
		theme.FilePanelBorder, theme.FilePanelBorderActive = "#c4c9ce", "#5c6f7f"
		theme.FilePanelTopDirectoryIcon, theme.FilePanelTopPath = "#47505a", "#3b4552"
		theme.DirectoryIconColor, theme.Cursor = "#51606b", "#303841"
	}
	common.SetConfig(config)
	common.Theme = theme //nolint:reassign // Scoped fixture theme; outer render fixture restores the exact prior state.
	icon.InitIcon(config.Nerdfont, theme.DirectoryIconColor)
	common.LoadThemeConfig()
	common.LoadPrerenderedVariables()
}

type printablePanelState struct {
	name     string
	focused  bool
	selected bool
	scrolled bool
}

func printableRenderFrames(t *testing.T) (string, int) {
	t.Helper()
	states := []printablePanelState{
		{name: "browser-focused", focused: true},
		{name: "browser-unfocused"},
		{name: "selected-focused", focused: true, selected: true},
		{name: "selected-unfocused", selected: true},
		{name: "browser-focused-scrolled", focused: true, scrolled: true},
		{name: "browser-unfocused-scrolled", scrolled: true},
		{name: "selected-focused-scrolled", focused: true, selected: true, scrolled: true},
		{name: "selected-unfocused-scrolled", selected: true, scrolled: true},
	}
	var frames strings.Builder
	count := 0
	for _, width := range []int{40, 120} {
		for _, state := range states {
			frame := printableRenderFrame(t, width, state)
			fmt.Fprintf(&frames, "width=%d/state=%s/bytes=%d:%s", width, state.name, len(frame), frame)
			count++
		}
	}
	return frames.String(), count
}

func printableRenderFrame(t *testing.T, width int, state printablePanelState) string {
	t.Helper()
	panel := newFilePanelRenderBenchmark(100)
	panel.UpdateDimensions(width, 20)
	panel.IsFocused = state.focused
	if !state.selected {
		panel.ResetSelected()
		panel.PanelMode = BrowserMode
	}
	// Cover a symlink and absent metadata in addition to the existing fixture's
	// directories, mixed extensions, Unicode names and fixed modification times.
	linkInfo := panel.element[6].Info.(renderBenchmarkFileInfo)
	linkInfo.mode = os.ModeSymlink | 0o777
	panel.element[6].Info = linkInfo
	panel.element[7].Info = nil
	if state.scrolled {
		panel.FocusIndex(panel.PanelElementHeight() + 2)
	}
	frame := panel.Render(state.focused)
	if frame == "" || frame != panel.Render(state.focused) {
		t.Fatalf("width=%d state=%s: frame must be nonempty and byte-deterministic", width, state.name)
	}
	return frame
}

func snapshotPrintableModeIcons() func() {
	return snapshotFilePanelRenderValues(
		&icon.Space, &icon.SuperfileIcon,
		&icon.Home, &icon.Desktop, &icon.Trash, &icon.Download, &icon.Documents,
		&icon.Pictures, &icon.Videos, &icon.Music, &icon.Templates, &icon.PublicShare,
		&icon.CompressFile, &icon.ExtractFile, &icon.Copy, &icon.Cut, &icon.Delete,
		&icon.Cursor, &icon.Browser, &icon.Select, &icon.Error, &icon.Warn, &icon.Done,
		&icon.InOperation, &icon.Directory, &icon.Search, &icon.SortAsc, &icon.SortDesc,
		&icon.Terminal, &icon.Pinned, &icon.Disk,
	)
}
