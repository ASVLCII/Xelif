package filepanel

import (
	"github.com/charmbracelet/x/ansi"

	"github.com/yorukot/superfile/src/internal/common"
)

const sortToolbarFullWidth = 30
const panelActionGapWidth = 2
const (
	toolbarSearchLabel        = "Search"
	toolbarCommandsLabel      = "Commands"
	toolbarSortMinimumWidth   = 56
	toolbarMinimumFilterWidth = 18
	toolbarActionsRow         = 2
	legacyFilterRow           = 3
	legacyHeaderRow           = 4
)

func (m *Model) SortActionLabel() string {
	if common.Config.PlainMode {
		return "Sort v"
	}
	if m.GetContentWidth() < sortToolbarFullWidth {
		return "Sort"
	}
	return "↑↓ Sort ▾"
}

func (m *Model) SortActionContentX() int {
	if m.WorkspaceChrome {
		return m.toolbarLayout().sortX
	}
	search, commands := m.searchActionLabels()
	return len(search) + len(commands) + 2*panelActionGapWidth
}

func (m *Model) SortActionAtContentX(x int) bool {
	if m.WorkspaceChrome && !m.toolbarLayout().showSort {
		return false
	}
	start := m.SortActionContentX()
	return x >= start && x < start+ansi.StringWidth(m.SortActionLabel()) && x < m.GetContentWidth()
}

func (m *Model) searchActionLabels() (string, string) {
	if m.GetContentWidth() < sortToolbarFullWidth {
		return "Find", "Menu"
	}
	return toolbarSearchLabel, toolbarCommandsLabel
}

// SearchActionAtContentX uses the same cell positions as the rendered controls.
// The regular search input below continues to filter the current directory.
func (m *Model) SearchActionAtContentX(x int) (bool, bool) {
	if m.WorkspaceChrome {
		g := m.toolbarLayout()
		return x >= g.searchX &&
				x < g.searchX+len(toolbarSearchLabel), x >= g.commandsX &&
				x < g.commandsX+len(toolbarCommandsLabel)
	}
	searchLabel, commandLabel := m.searchActionLabels()
	if x < 0 || x >= m.GetContentWidth() {
		return false, false
	}
	commandX := len(searchLabel) + panelActionGapWidth
	return x < len(searchLabel), x >= commandX && x < commandX+len(commandLabel)
}

type toolbarGeometry struct {
	searchX, commandsX, sortX int
	showSort                  bool
}

func (m *Model) toolbarLayout() toolbarGeometry {
	width := m.GetContentWidth()
	showSort := width >= toolbarSortMinimumWidth
	actions := len(toolbarSearchLabel) + panelActionGapWidth + len(toolbarCommandsLabel)
	if showSort {
		actions += panelActionGapWidth + ansi.StringWidth(m.SortActionLabel())
	}
	start := max(toolbarMinimumFilterWidth, width-actions)
	commandsX := start + len(toolbarSearchLabel) + panelActionGapWidth
	return toolbarGeometry{
		searchX:   start,
		commandsX: commandsX,
		sortX:     commandsX + len(toolbarCommandsLabel) + panelActionGapWidth,
		showSort:  showSort,
	}
}

func (m *Model) FilterTextWidth() int {
	if m.WorkspaceChrome {
		return max(1, m.toolbarLayout().searchX-panelActionGapWidth)
	}
	return m.width - common.InnerPadding
}

func (m *Model) ActionsRow() int { return toolbarActionsRow }
func (m *Model) FilterRow() int {
	if m.WorkspaceChrome {
		return toolbarActionsRow
	}
	return legacyFilterRow
}
func (m *Model) ColumnHeaderRow() int {
	if m.WorkspaceChrome {
		return toolbarActionsRow + 1
	}
	return legacyHeaderRow
}
