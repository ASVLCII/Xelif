package globalsearch

// Scroll moves through results without wrapping, for wheel and page navigation.
func (m *Model) Scroll(delta int) {
	if !m.open || len(m.results) == 0 {
		return
	}
	m.cursor = min(max(m.cursor+delta, 0), len(m.results)-1)
	m.updateRenderIndex()
}

func (m *Model) navigateUp() {
	if len(m.results) == 0 {
		return
	}
	if m.cursor == 0 {
		m.cursor = len(m.results) - 1
	} else {
		m.cursor--
	}
	m.updateRenderIndex()
}

func (m *Model) navigateDown() {
	if len(m.results) == 0 {
		return
	}
	if m.cursor == len(m.results)-1 {
		m.cursor = 0
	} else {
		m.cursor++
	}
	m.updateRenderIndex()
}

func (m *Model) updateRenderIndex() {
	visible := m.visibleResultCount()
	if m.cursor < m.renderIndex {
		m.renderIndex = m.cursor
	} else if m.cursor >= m.renderIndex+visible {
		m.renderIndex = m.cursor - visible + 1
	}
	maxStart := max(0, len(m.results)-visible)
	m.renderIndex = min(max(m.renderIndex, 0), maxStart)
}
