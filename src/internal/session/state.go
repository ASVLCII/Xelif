// Package session persists versioned application sessions and named workspaces.
package session

// CurrentVersion is the only on-disk schema version understood by this package.
const CurrentVersion = 1

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
