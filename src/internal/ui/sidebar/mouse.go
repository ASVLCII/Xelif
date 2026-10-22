package sidebar

const sidebarSearchScreenY = 3

// IsSearchRowY reports whether y is the currently rendered sidebar search row.
func (s *Model) IsSearchRowY(y int, _ bool) bool {
	return y >= 0 && y < s.height && y == sidebarSearchScreenY
}

// FocusDirectoryAtY focuses the directory rendered at zero-based screen row y.
func (s *Model) FocusDirectoryAtY(y int, _ bool) (string, bool) {
	if y < 0 || y >= s.height {
		return "", false
	}

	firstDirectoryY := sidebarSearchScreenY + 1
	if y < firstDirectoryY || s.renderIndex < 0 || s.renderIndex >= len(s.directories) {
		return "", false
	}

	row := y - firstDirectoryY
	if row >= s.viewportRows() {
		return "", false
	}
	row += s.renderOffset
	for i := s.renderIndex; i < len(s.directories); i++ {
		requiredHeight := s.directories[i].requiredHeight()
		if row < requiredHeight {
			if s.directories[i].isDivider() {
				return "", false
			}
			s.cursor = i
			return s.directories[i].Location, true
		}
		row -= requiredHeight
	}
	return "", false
}
