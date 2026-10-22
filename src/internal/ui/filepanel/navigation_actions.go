package filepanel

const panelSearchActions = "Search  Commands"

func (m *Model) searchActionLabels() (string, string) {
	if m.GetContentWidth() < len(panelSearchActions) {
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
