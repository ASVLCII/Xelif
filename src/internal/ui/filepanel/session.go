package filepanel

import (
	"os"

	"github.com/yorukot/superfile/src/internal/session"
	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

// SnapshotSession returns the durable navigation state of the panel.
func (m *Model) SnapshotSession() session.PanelState {
	tabs := make([]session.TabState, len(m.tabs))
	for index, tab := range m.tabs {
		tabs[index] = session.TabState{
			Location:       tab.location,
			BackHistory:    append([]string{}, tab.backHistory...),
			ForwardHistory: append([]string{}, tab.forwardHistory...),
		}
	}
	return session.PanelState{
		Tabs:         tabs,
		ActiveTab:    m.activeTab,
		SortKind:     int(m.SortKind),
		SortReversed: m.SortReversed,
	}
}

// RestoreSession constructs a panel from durable state, pruning locations that
// are no longer directories. fallbackLocation is retained when no tab survives.
func RestoreSession(state session.PanelState, fallbackLocation string, focused bool) Model {
	tabs := make([]panelTab, 0, len(state.Tabs))
	activeTab := -1
	for index, tabState := range state.Tabs {
		if !isSessionDirectory(tabState.Location) {
			continue
		}
		if index == state.ActiveTab {
			activeTab = len(tabs)
		}
		tabs = append(tabs, panelTab{
			location:       tabState.Location,
			backHistory:    validSessionDirectories(tabState.BackHistory),
			forwardHistory: validSessionDirectories(tabState.ForwardHistory),
		})
	}
	if len(tabs) == 0 {
		tabs = append(tabs, panelTab{location: fallbackLocation})
	}
	if activeTab < 0 {
		activeTab = min(max(state.ActiveTab, 0), len(tabs)-1)
	}

	sortKind := sortmodel.SortKind(state.SortKind)
	if sortKind < sortmodel.SortByName || sortKind > sortmodel.SortByNatural {
		sortKind = sortmodel.SortByName
	}
	panel := New(tabs[activeTab].location, focused, "", sortKind, state.SortReversed)
	panel.tabs = tabs
	panel.activeTab = activeTab
	return panel
}

func validSessionDirectories(locations []string) []string {
	valid := make([]string, 0, len(locations))
	for _, location := range locations {
		if isSessionDirectory(location) {
			valid = append(valid, location)
		}
	}
	return valid
}

func isSessionDirectory(location string) bool {
	info, err := os.Stat(location)
	return err == nil && info.IsDir()
}
