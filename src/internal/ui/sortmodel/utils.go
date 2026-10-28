package sortmodel

func (m *Model) IsOpen() bool {
	return m.open
}

func (m *Model) Open(curSortKind SortKind, reversed ...bool) {
	m.SelectedKind = curSortKind
	m.Reversed = len(reversed) > 0 && reversed[0]
	m.Cursor = 0
	for index, kind := range menuKinds {
		if kind == curSortKind {
			m.Cursor = index
		}
	}
	m.open = true
}

func (m *Model) Close() {
	m.open = false
	m.Cursor = 0
}

func (m *Model) GetSelectedKind() SortKind {
	if m.Cursor >= 0 && m.Cursor < len(menuKinds) {
		return menuKinds[m.Cursor]
	}
	return m.SelectedKind
}

func (m *Model) GetSelectedReversed() bool {
	if m.Cursor >= len(menuKinds) {
		return m.Cursor == len(menuKinds)+1
	}
	return m.Reversed
}

// ItemAtRow skips the noninteractive separator in the dropdown.
func (m *Model) ItemAtRow(row int) (int, bool) {
	if row == sortFieldCount || row < 0 || row >= sortMenuRowCount {
		return 0, false
	}
	if row > sortFieldCount {
		row--
	}
	return row, true
}
