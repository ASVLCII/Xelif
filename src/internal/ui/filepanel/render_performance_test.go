package filepanel

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yorukot/superfile/src/config/icon"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

// These fixtures intentionally avoid directory reads during rendering, so the
// benchmarks measure frame construction rather than filesystem throughput.
func BenchmarkFilePanelRender(b *testing.B) {
	prepareFilePanelRenderBenchmark(b)
	for _, count := range []int{100, 10000} {
		b.Run(fmt.Sprintf("entries_%d", count), func(b *testing.B) {
			panel := newFilePanelRenderBenchmark(count)
			b.ReportAllocs()
			var frame string
			for b.Loop() {
				frame = panel.Render(true)
			}
			if frame == "" {
				b.Fatal("rendered frame is empty")
			}
			runtime.KeepAlive(frame)
		})
	}
}

func BenchmarkFilePanelScrollRender(b *testing.B) {
	prepareFilePanelRenderBenchmark(b)
	for _, count := range []int{100, 10000} {
		b.Run(fmt.Sprintf("entries_%d", count), func(b *testing.B) {
			panel := newFilePanelRenderBenchmark(count)
			b.ReportAllocs()
			var frame string
			for b.Loop() {
				panel.ListDown()
				frame = panel.Render(true)
			}
			if frame == "" {
				b.Fatal("rendered frame is empty")
			}
			runtime.KeepAlive(frame)
		})
	}
}

func TestFilePanelRenderOutputFingerprint(t *testing.T) {
	prepareFilePanelRenderBenchmark(t)
	for _, width := range []int{40, 120} {
		for _, state := range []string{"unselected", "selected", "scrolled"} {
			t.Run(fmt.Sprintf("width_%d/%s", width, state), func(t *testing.T) {
				panel := newFilePanelRenderBenchmark(100)
				panel.UpdateDimensions(width, 20)
				switch state {
				case "unselected":
					panel.ResetSelected()
					panel.PanelMode = BrowserMode
				case "scrolled":
					panel.FocusIndex(panel.PanelElementHeight())
				}
				frame := panel.Render(true)
				if frame == "" || frame != panel.Render(true) {
					t.Fatal("rendered frame must be nonempty and deterministic")
				}
				if !strings.Contains(frame, "\x1b[") || !strings.Contains(frame, "38;2;") {
					t.Fatal("rendered frame must contain truecolor ANSI styling")
				}
				t.Logf("SHA256 %x", sha256.Sum256([]byte(frame)))
			})
		}
	}
}

func prepareFilePanelRenderBenchmark(b testing.TB) {
	b.Helper()
	b.Cleanup(snapshotFilePanelRenderGlobals())
	b.Setenv("NO_COLOR", "")
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		b.Fatal(err)
	}
	// Decode into fresh structs: TOML may reuse existing slice backing arrays,
	// which would otherwise also mutate the state captured for cleanup.
	//nolint:reassign // Scoped fixture setup must not share captured slice backing arrays.
	common.Config, common.Theme, common.Hotkeys = common.ConfigType{}, common.ThemeType{}, common.HotkeysType{}
	if err := common.PopulateGlobalConfigs(); err != nil {
		b.Fatal(err)
	}
	common.Config.FilePanelExtraColumns = 2
	common.Config.FilePanelNamePercent = 50
	common.Config.ColorFileIcons = true
	common.Config.PlainMode = false
	common.Config.TransparentBackground = false
	common.LoadThemeConfig()
	common.LoadPrerenderedVariables()
}

// Keep this fixture scoped even when the package started with uninitialized
// globals or another test supplied custom styles and pre-rendered strings.
func snapshotFilePanelRenderGlobals() func() {
	oldConfig, oldTheme, oldHotkeys := common.Config, common.Theme, common.Hotkeys
	oldFolderIcon := icon.Folders["folder"]
	restoreStrings := snapshotFilePanelRenderValues(
		&common.BottomMiddleBorderSplit, &common.LipglossError,
		&common.SideBarSuperfileTitle, &common.SideBarHomeDivider,
		&common.SideBarRecentDivider, &common.SideBarPinnedDivider,
		&common.SideBarDisksDivider, &common.SideBarNoneText,
		&common.ProcessBarNoneText, &common.ClipboardNoneText,
		&common.FilePanelTopDirectoryIcon, &common.FilePanelNoneText,
		&common.FilePreviewNoFileInfoText, &common.FilePreviewNoContentText,
		&common.FilePreviewUnsupportedFormatText, &common.FilePreviewUnsupportedFileMode,
		&common.FilePreviewDirectoryUnreadableText, &common.FilePreviewEmptyText,
		&common.FilePreviewError, &common.FilePreviewPanelClosedText,
		&common.FilePreviewImagePreviewDisabledText, &common.FilePreviewUnsupportedImageFormatsText,
		&common.FilePreviewImageConversionErrorText, &common.FilePreviewBatNotInstalledText,
		&common.FilePreviewThumbnailGenerationErrorText,
		&common.CheckboxChecked, &common.CheckboxCheckedFocused,
		&common.CheckboxEmpty, &common.CheckboxEmptyFocused,
		&common.ModalConfirmInputText, &common.ModalCancelInputText,
		&common.ModalOkayInputText, &common.ModalInputSpacingText,
	)
	restoreStyles := snapshotFilePanelRenderValues(
		&common.TerminalTooSmall, &common.TerminalCorrectSize,
		&common.MainStyle, &common.FilePanelStyle, &common.SidebarStyle,
		&common.FooterStyle, &common.ModalStyle,
		&common.SidebarDividerStyle, &common.SidebarTitleStyle, &common.SidebarSelectedStyle,
		&common.FilePanelCursorStyle, &common.FooterCursorStyle, &common.ModalCursorStyle,
		&common.FilePanelTopDirectoryIconStyle, &common.FilePanelTopPathStyle,
		&common.FilePanelItemSelectedStyle, &common.FilePanelSelectBoxStyle,
		&common.ProcessErrorStyle, &common.ProcessInOperationStyle,
		&common.ProcessCancelStyle, &common.ProcessSuccessfulStyle,
		&common.ModalCancel, &common.ModalConfirm, &common.ModalTitleStyle, &common.ModalErrorStyle,
		&common.HelpMenuHotkeyStyle, &common.HelpMenuTitleStyle,
		&common.PromptSuccessStyle, &common.PromptFailureStyle,
	)
	restoreColors := snapshotFilePanelRenderValues(
		&common.FilePanelBorderColor, &common.SidebarBorderColor, &common.FooterBorderColor,
		&common.FilePanelBorderActiveColor, &common.SidebarBorderActiveColor,
		&common.FooterBorderActiveColor, &common.ModalBorderActiveColor,
		&common.FullScreenBGColor, &common.FilePanelBGColor, &common.SidebarBGColor,
		&common.FooterBGColor, &common.ModalBGColor,
		&common.FullScreenFGColor, &common.FilePanelFGColor, &common.SidebarFGColor,
		&common.FooterFGColor, &common.ModalFGColor,
	)
	return func() {
		//nolint:reassign // Restore the process-wide configuration after the scoped render fixture.
		common.Config, common.Theme, common.Hotkeys = oldConfig, oldTheme, oldHotkeys
		icon.Folders["folder"] = oldFolderIcon
		// Rebuild common's private cached theme colors, then restore the exact
		// exported state rather than relying on initialized hotkey slices.
		common.LoadThemeConfig()
		restoreColors()
		restoreStyles()
		restoreStrings()
	}
}

func snapshotFilePanelRenderValues[T any](targets ...*T) func() {
	values := make([]T, len(targets))
	for index, target := range targets {
		values[index] = *target
	}
	return func() {
		for index, target := range targets {
			*target = values[index]
		}
	}
}

func newFilePanelRenderBenchmark(count int) *Model {
	root := filepath.Join(string(filepath.Separator), "xelif-render-benchmark")
	panel := New(root, true, "", sortmodel.SortByName, false)
	panel.UpdateDimensions(120, 40)
	panel.PanelMode = SelectMode
	panel.element = make([]Element, count)
	for index := range panel.element {
		extensions := [...]string{"go", "png", "txt", "zip", "mp3", "bin"}
		name := fmt.Sprintf("file-%05d.%s", index, extensions[index%len(extensions)])
		if index%2 != 0 {
			name = fmt.Sprintf("tài-liệu-文件-%05d.%s", index, extensions[index%len(extensions)])
		}
		mode := os.FileMode(0o644)
		if index%10 == 0 {
			mode = os.ModeDir | 0o755
		}
		location := filepath.Join(root, name)
		panel.element[index] = Element{
			Name:      name,
			Location:  location,
			Directory: mode.IsDir(),
			Info:      renderBenchmarkFileInfo{name: name, mode: mode, size: int64(index) * 1024},
		}
		if index%4 == 0 {
			panel.SetSelected(location)
		}
	}
	return &panel
}

type renderBenchmarkFileInfo struct {
	name string
	mode os.FileMode
	size int64
}

func (info renderBenchmarkFileInfo) Name() string      { return info.name }
func (info renderBenchmarkFileInfo) Size() int64       { return info.size }
func (info renderBenchmarkFileInfo) Mode() os.FileMode { return info.mode }
func (info renderBenchmarkFileInfo) ModTime() time.Time {
	return time.Unix(1700000000, 0).UTC()
}
func (info renderBenchmarkFileInfo) IsDir() bool { return info.mode.IsDir() }
func (info renderBenchmarkFileInfo) Sys() any    { return nil }
