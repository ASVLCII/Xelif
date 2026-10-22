// Package textviewer provides a small read-only, scrollable text overlay.
package textviewer

import (
	"fmt"
	"strings"

	"github.com/yorukot/superfile/src/internal/i18n"
	"github.com/yorukot/superfile/src/internal/ui"
)

const (
	minimumWidth    = 12
	minimumHeight   = 4
	frameBorderRows = 2
)

type Model struct {
	open   bool
	title  string
	lines  []string
	offset int
	width  int
	height int
}

func New() Model { return Model{width: minimumWidth, height: minimumHeight} }

func (m *Model) Open(title, content string) {
	m.open = true
	m.title = strings.TrimSpace(title)
	content = strings.ReplaceAll(content, "\r\n", "\n")
	m.lines = strings.Split(content, "\n")
	if len(m.lines) == 0 {
		m.lines = []string{""}
	}
	m.offset = 0
}

func (m *Model) Close()         { m.open = false }
func (m *Model) IsOpen() bool   { return m.open }
func (m *Model) GetWidth() int  { return m.width }
func (m *Model) GetHeight() int { return m.height }

func (m *Model) SetDimensions(width, height int) {
	m.width = max(width, minimumWidth)
	m.height = max(height, minimumHeight)
	m.clampOffset()
}

func (m *Model) Scroll(delta int) {
	m.offset += delta
	m.clampOffset()
}

func (m *Model) Page(delta int) {
	m.Scroll(delta * max(m.contentHeight()-1, 1))
}

func (m *Model) Home() { m.offset = 0 }

func (m *Model) End() {
	m.offset = m.maxOffset()
}

// HandleClick closes the viewer when its bottom-left "Close" affordance is clicked.
func (m *Model) HandleClick(localX, localY int) bool {
	if !m.open || localY != m.height-1 || localX < 0 || localX > 12 {
		return false
	}
	m.Close()
	return true
}

func (m *Model) Render() string {
	renderer := ui.PromptRenderer(m.height, m.width)
	title := m.title
	if title == "" {
		title = i18n.T("details")
	}
	renderer.SetBorderTitle(title)
	start := min(m.offset, len(m.lines))
	end := min(start+m.contentHeight(), len(m.lines))
	for _, line := range m.lines[start:end] {
		renderer.AddLines(line)
	}
	renderer.SetBorderInfoItems(
		i18n.T("close"),
		fmt.Sprintf("%d-%d/%d", visibleStart(start, len(m.lines)), end, len(m.lines)),
	)
	return renderer.Render()
}

func (m *Model) contentHeight() int { return max(m.height-frameBorderRows, 1) }
func (m *Model) maxOffset() int     { return max(len(m.lines)-m.contentHeight(), 0) }

func (m *Model) clampOffset() {
	m.offset = min(max(m.offset, 0), m.maxOffset())
}

func visibleStart(start, total int) int {
	if total == 0 {
		return 0
	}
	return start + 1
}
