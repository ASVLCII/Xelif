package internal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/textviewer"
	stringfunction "github.com/yorukot/superfile/src/pkg/string_function"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestLintCleanupOverlayPlacementPreservesRounding(t *testing.T) {
	for _, dimensions := range [][4]int{{120, 40, 80, 20}, {121, 41, 81, 21}, {40, 17, 81, 21}} {
		m := &model{fullWidth: dimensions[0], fullHeight: dimensions[1], textViewer: textviewer.New()}
		m.textViewer.SetDimensions(dimensions[2], dimensions[3])
		m.textViewer.Open("Preview", "Unicode 文件\nSecond line")
		background := strings.Repeat(strings.Repeat(" ", m.fullWidth)+"\n", m.fullHeight)
		// This is the original placement formula, including separate truncation.
		want := stringfunction.PlaceOverlay(m.fullWidth/2-m.textViewer.GetWidth()/2,
			m.fullHeight/2-m.textViewer.GetHeight()/2, m.textViewer.Render(), background)
		require.Equal(t, want, m.updateRenderForOverlay(background))
	}
}

func TestLintCleanupOverlappingNavigationBindingsKeepPriority(t *testing.T) {
	originalDown, originalNewTab := common.Hotkeys.ListDown, common.Hotkeys.NewFileTab
	t.Cleanup(func() { common.Hotkeys.ListDown, common.Hotkeys.NewFileTab = originalDown, originalNewTab })
	common.Hotkeys.ListDown, common.Hotkeys.NewFileTab = []string{"test-overlap"}, []string{"test-overlap"}
	directory := t.TempDir()
	utils.SetupFiles(t, directory+"/one.txt", directory+"/two.txt")
	m := defaultTestModel(directory)
	panel := m.getFocusedFilePanel()
	m.mainKey("test-overlap")
	require.Equal(t, 1, panel.GetCursor())
	require.Equal(t, 1, panel.TabCount(), "list navigation must win over a later tab binding")
}

func TestLintCleanupOverlappingTabBindingsKeepPriority(t *testing.T) {
	originalNewTab, originalBack := common.Hotkeys.NewFileTab, common.Hotkeys.HistoryBack
	t.Cleanup(func() { common.Hotkeys.NewFileTab, common.Hotkeys.HistoryBack = originalNewTab, originalBack })
	common.Hotkeys.NewFileTab, common.Hotkeys.HistoryBack = []string{"test-overlap"}, []string{"test-overlap"}
	m := defaultTestModel(t.TempDir())
	m.mainKey("test-overlap")
	require.Equal(t, 2, m.getFocusedFilePanel().TabCount(), "new tab must win over a later history binding")
	require.False(t, m.notifyModel.IsOpen())
}
