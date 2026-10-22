package internal

import (
	"log/slog"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yorukot/superfile/src/internal/common"
)

func (m *model) setTextViewerSize() {
	m.textViewer.SetDimensions(3*m.fullWidth/4, 3*m.fullHeight/4) //nolint:mnd // Details favor readable space.
}

// Failures must replace the progress surface, not hide a notification behind
// it. Error paths can contain untrusted filenames: strip controls before wrap.
func (m *model) showTextViewerError(title string, err error, summary string) {
	slog.Error(title, "error", err)
	content := err.Error()
	if summary != "" {
		content = summary + "\n\n" + content
	}
	m.setTextViewerSize()
	content = common.MakePrintableWithEscCheck(content, false)
	m.textViewer.Open(title, ansi.Hardwrap(content, max(m.textViewer.GetWidth()-common.BorderPadding, 1), false))
}

func (m *model) handleTextViewerKey(key string) {
	switch {
	case slices.Contains(common.Hotkeys.Quit, key), slices.Contains(common.Hotkeys.CancelTyping, key):
		m.textViewer.Close()
	case slices.Contains(common.Hotkeys.ListUp, key):
		m.textViewer.Scroll(-1)
	case slices.Contains(common.Hotkeys.ListDown, key):
		m.textViewer.Scroll(1)
	case slices.Contains(common.Hotkeys.PageUp, key):
		m.textViewer.Page(-1)
	case slices.Contains(common.Hotkeys.PageDown, key):
		m.textViewer.Page(1)
	case key == "home":
		m.textViewer.Home()
	case key == "end":
		m.textViewer.End()
	}
}

func (m *model) handleTextViewerClick(mouse tea.Mouse) {
	overlayX := m.fullWidth/common.CenterDivisor - m.textViewer.GetWidth()/common.CenterDivisor
	overlayY := m.fullHeight/common.CenterDivisor - m.textViewer.GetHeight()/common.CenterDivisor
	if mouse.X < overlayX || mouse.X >= overlayX+m.textViewer.GetWidth() ||
		mouse.Y < overlayY || mouse.Y >= overlayY+m.textViewer.GetHeight() {
		return
	}
	m.textViewer.HandleClick(mouse.X-overlayX, mouse.Y-overlayY)
}
