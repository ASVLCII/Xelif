package commandpalette

import "charm.land/bubbles/v2/textinput"

// Item is one command available in the palette.
type Item struct {
	ID             string
	Label          string
	Category       string
	Keywords       []string
	Hotkeys        []string
	Disabled       bool
	DisabledReason string
}

// ExecuteAction asks the parent model to execute a command by ID.
type ExecuteAction struct {
	ID string
}

func (a ExecuteAction) String() string {
	return "ExecuteAction for " + a.ID
}

// Model contains the command-palette modal state.
type Model struct {
	open        bool
	items       []Item
	filtered    []Item
	textInput   textinput.Model
	cursor      int
	renderIndex int
	width       int
	maxHeight   int
}
