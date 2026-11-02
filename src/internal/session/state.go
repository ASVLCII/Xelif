// Package session persists versioned application sessions and named workspaces.
package session

// CurrentVersion is the schema version of a single layout and named preset.
const CurrentVersion = 1

// CurrentAppVersion is the schema version of the multi-workspace last session.
const CurrentAppVersion = 2

// Durable-state limits prevent malformed files from describing unbounded state.
// Locations are checked structurally, never against the currently mounted drives.
const (
	MaxWorkspaces          = 128
	MaxPanels              = 64
	MaxTabsPerPanel        = 256
	MaxHistoryEntries      = 4096
	MaxLocationBytes       = 32768
	MaxWorkspaceIDBytes    = 128
	MaxWorkspaceTitleBytes = 256
	MaxStateFileBytes      = 16 << 20
)

// AppState is the ordered collection of independent open workspaces.
type AppState struct {
	Version           int              `json:"version"`
	ActiveWorkspaceID string           `json:"active_workspace_id"`
	Workspaces        []WorkspaceState `json:"workspaces"`
}

// WorkspaceState contains a stable identity, display title, and durable layout.
// Layout deliberately excludes selections, dialogs, and in-flight operations.
type WorkspaceState struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Layout State  `json:"layout"`
}

// State is the durable, UI-independent representation of an application session.
type State struct {
	Version        int          `json:"version"`
	FocusedPanel   int          `json:"focused_panel"`
	ShowDotFiles   bool         `json:"show_dot_files"`
	PreviewVisible bool         `json:"preview_visible"`
	FooterVisible  bool         `json:"footer_visible"`
	Panels         []PanelState `json:"panels"`
}

// PanelState is the durable state of one file panel.
type PanelState struct {
	Tabs         []TabState `json:"tabs"`
	ActiveTab    int        `json:"active_tab"`
	SortKind     int        `json:"sort_kind"`
	SortReversed bool       `json:"sort_reversed"`
}

// TabState is the durable navigation state of one panel tab.
type TabState struct {
	Location       string   `json:"location"`
	BackHistory    []string `json:"back_history"`
	ForwardHistory []string `json:"forward_history"`
}

// CloneState returns an independent copy, including every navigation history.
func CloneState(state State) State {
	if state.Panels != nil {
		state.Panels = append([]PanelState{}, state.Panels...)
	}
	for index := range state.Panels {
		panel := &state.Panels[index]
		if panel.Tabs != nil {
			panel.Tabs = append([]TabState{}, panel.Tabs...)
		}
		for tabIndex := range panel.Tabs {
			tab := &panel.Tabs[tabIndex]
			if tab.BackHistory != nil {
				tab.BackHistory = append([]string{}, tab.BackHistory...)
			}
			if tab.ForwardHistory != nil {
				tab.ForwardHistory = append([]string{}, tab.ForwardHistory...)
			}
		}
	}
	return state
}

// CloneAppState returns an independent copy of all workspace layouts.
func CloneAppState(state AppState) AppState {
	if state.Workspaces != nil {
		state.Workspaces = append([]WorkspaceState{}, state.Workspaces...)
	}
	for index := range state.Workspaces {
		state.Workspaces[index].Layout = CloneState(state.Workspaces[index].Layout)
	}
	return state
}
