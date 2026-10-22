package internal

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/helpmenu"
	"github.com/yorukot/superfile/src/internal/ui/processbar"
	"github.com/yorukot/superfile/src/internal/ui/textviewer"
)

func BenchmarkMainNavigationDispatch(b *testing.B) {
	m := &model{focusPanel: processBarFocus, processBarModel: processbar.New()}
	key := common.Hotkeys.ListDown[0]
	b.ReportAllocs()
	for b.Loop() {
		m.mainKey(key)
	}
}

func BenchmarkHelpWheelDispatch(b *testing.B) {
	m := &model{helpMenu: helpmenu.New()}
	m.helpMenu.Open()
	message := tea.MouseWheelMsg{Button: tea.MouseWheelDown}
	b.ReportAllocs()
	for b.Loop() {
		m.handleMouseMsg(message)
	}
}

func BenchmarkRootOverlayRender(b *testing.B) {
	m := &model{fullWidth: 120, fullHeight: 40, textViewer: textviewer.New()}
	m.textViewer.SetDimensions(80, 20)
	m.textViewer.Open("Preview", strings.Repeat("Example text\n", 40))
	background := strings.Repeat(strings.Repeat(" ", 120)+"\n", 40)
	b.ReportAllocs()
	for b.Loop() {
		m.updateRenderForOverlay(background)
	}
}
