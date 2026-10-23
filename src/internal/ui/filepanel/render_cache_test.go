package filepanel

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui"
)

// Compare with independently rendering every column. Reuse the same panel
// across mutations to catch caches that outlive the frame they describe.
func TestFileEntriesMatchUncachedRendering(t *testing.T) {
	prepareFilePanelRenderBenchmark(t)
	panel := newFilePanelRenderBenchmark(100)
	for _, plain := range []bool{false, true} {
		common.Config.PlainMode = plain
		for _, width := range []int{12, 40, 120, 200} {
			for _, extra := range []int{0, 1, 2, 3} {
				common.Config.FilePanelExtraColumns = extra
				panel.UpdateDimensions(width, 20)
				label := fmt.Sprintf("plain_%t/width_%d/extra_%d", plain, width, extra)
				checkEntryFrameVariants(t, panel, label)
			}
		}
	}
}

func checkEntryFrameVariants(t *testing.T, panel *Model, label string) {
	t.Helper()
	for _, selected := range []bool{false, true, false} {
		panel.ResetSelected()
		if selected {
			for index := 0; index < panel.ElemCount(); index += 2 {
				panel.SetSelected(panel.GetElementAtIdx(index).Location)
			}
		}
		for _, focused := range []bool{false, true} {
			name := fmt.Sprintf("%s/selected_%t/focused_%t", label, selected, focused)
			t.Run(name, func(t *testing.T) {
				checkEntryFrame(t, panel, focused)
			})
		}
	}
}

func checkEntryFrame(t *testing.T, panel *Model, focused bool) {
	t.Helper()
	actual := ui.FilePanelRenderer(panel.height, panel.width, focused)
	expected := ui.FilePanelRenderer(panel.height, panel.width, focused)
	panel.renderFileEntries(actual)
	end := min(panel.renderIndex+panel.PanelElementHeight(), panel.ElemCount())
	for index := panel.renderIndex; index < end; index++ {
		var row strings.Builder
		for _, column := range panel.columns {
			row.WriteString(column.Render(index))
		}
		expected.AddLines(row.String())
	}
	if actual.Render() != expected.Render() {
		t.Fatal("cached frame differs from independently styled rows")
	}
	// Move the viewport before the next frame.
	panel.ListDown()
}
