package filemodel

import (
	"os"

	"github.com/yorukot/superfile/src/internal/session"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
)

// SnapshotSession returns independent copies of the durable file-model state.
func (m *Model) SnapshotSession() ([]session.PanelState, int, bool) {
	panels := make([]session.PanelState, len(m.FilePanels))
	for index := range m.FilePanels {
		panels[index] = m.FilePanels[index].SnapshotSession()
	}
	return panels, m.FocusedPanelIndex, m.DisplayDotFiles
}

// RestoreSession replaces file panels with valid durable state. Missing panels
// are pruned, at least one fallback panel is retained, and focus is repaired.
func (m *Model) RestoreSession(
	states []session.PanelState,
	focusedPanel int,
	showDotFiles bool,
	fallbackLocation string,
) {
	limit := m.MaxFilePanel
	if limit < 1 {
		limit = 1
	}
	panels := make([]filepanel.Model, 0, min(len(states), limit))
	restoredFocus := -1
	for index, state := range states {
		if len(panels) == limit {
			break
		}
		if !hasValidSessionTab(state) {
			continue
		}
		if index == focusedPanel {
			restoredFocus = len(panels)
		}
		panels = append(panels, filepanel.RestoreSession(state, fallbackLocation, false))
	}
	if len(panels) == 0 {
		panels = append(panels, filepanel.RestoreSession(session.PanelState{}, fallbackLocation, false))
	}
	if restoredFocus < 0 {
		restoredFocus = min(max(focusedPanel, 0), len(panels)-1)
	}
	for index := range panels {
		panels[index].IsFocused = index == restoredFocus
	}

	m.FilePanels = panels
	m.FocusedPanelIndex = restoredFocus
	m.DisplayDotFiles = showDotFiles
}

func hasValidSessionTab(state session.PanelState) bool {
	for _, tab := range state.Tabs {
		info, err := os.Stat(tab.Location)
		if err == nil && info.IsDir() {
			return true
		}
	}
	return false
}
