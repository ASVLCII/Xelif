package filepanel

import "github.com/charmbracelet/x/ansi"

const sortToolbarFullWidth = 30
const panelActionGapWidth = 2

func (m *Model) SortActionLabel() string {
	if m.GetContentWidth() < sortToolbarFullWidth {
		return "Sort"
	}
	return "↑↓ Sort ▾"
}

func (m *Model) SortActionContentX() int {
	search, commands := m.searchActionLabels()
	return len(search) + len(commands) + 2*panelActionGapWidth
}

func (m *Model) SortActionAtContentX(x int) bool {
	start := m.SortActionContentX()
	return x >= start && x < start+ansi.StringWidth(m.SortActionLabel()) && x < m.GetContentWidth()
}

func (m *Model) searchActionLabels() (string, string) {
	if m.GetContentWidth() < sortToolbarFullWidth {
		return "Find", "Menu"
	}
	return "Search", "Commands"
}

// SearchActionAtContentX uses the same cell positions as the rendered controls.
// The regular search input below continues to filter the current directory.
func (m *Model) SearchActionAtContentX(x int) (bool, bool) {
	searchLabel, commandLabel := m.searchActionLabels()
	if x < 0 || x >= m.GetContentWidth() {
		return false, false
	}
	return x < len(searchLabel), x >= len(searchLabel)+2 && x < len(searchLabel)+2+len(commandLabel)
}
