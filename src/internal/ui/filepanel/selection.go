package filepanel

// SelectRangeTo selects the inclusive visible range between the selection
// anchor and target index. The first successful call anchors at the cursor.
func (m *Model) SelectRangeTo(index int) bool {
	if index < 0 || index >= m.ElemCount() || m.cursor < 0 || m.cursor >= m.ElemCount() {
		return false
	}

	if m.selectionAnchor == "" {
		m.selectionAnchor = m.element[m.cursor].Location
	}
	anchorIndex := m.FindElementIndexByLocation(m.selectionAnchor)
	if anchorIndex < 0 {
		m.selectionAnchor = m.element[m.cursor].Location
		anchorIndex = m.cursor
	}

	start, end := anchorIndex, index
	if start > end {
		start, end = end, start
	}
	for _, element := range m.element[start : end+1] {
		if !m.CheckSelected(element.Location) {
			m.SetSelected(element.Location)
		}
	}
	return true
}

// InvertVisibleSelection toggles selection for each element in the current
// filtered and sorted view. Selections outside the view are unchanged.
func (m *Model) InvertVisibleSelection() {
	for _, element := range m.element {
		m.ToggleSelected(element.Location)
	}
}

// SelectVisibleWhere selects visible elements accepted by predicate.
func (m *Model) SelectVisibleWhere(predicate func(Element) bool) {
	if predicate == nil {
		return
	}
	for _, element := range m.element {
		if predicate(element) && !m.CheckSelected(element.Location) {
			m.SetSelected(element.Location)
		}
	}
}

// SelectVisibleFiles selects all visible non-directory elements.
func (m *Model) SelectVisibleFiles() {
	m.SelectVisibleWhere(func(element Element) bool {
		return !element.Directory
	})
}

// SelectVisibleDirectories selects all visible directory elements.
func (m *Model) SelectVisibleDirectories() {
	m.SelectVisibleWhere(func(element Element) bool {
		return element.Directory
	})
}

// ClearSelectionAnchor starts the next range selection from the cursor.
func (m *Model) ClearSelectionAnchor() {
	m.selectionAnchor = ""
}

func (m *Model) clearMissingSelectionAnchor() {
	if m.selectionAnchor != "" && m.FindElementIndexByLocation(m.selectionAnchor) < 0 {
		m.ClearSelectionAnchor()
	}
}
