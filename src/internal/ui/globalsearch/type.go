package globalsearch

import (
	"context"

	"charm.land/bubbles/v2/textinput"
)

const (
	ProviderEverything = "Everything"
	ProviderLocal      = "Local fallback"
)

// Result is a file or directory returned by global search.
type Result struct {
	Path      string
	Directory bool
}

// Response contains one completed search request.
type Response struct {
	Results  []Result
	Provider string
	Partial  bool
	Notice   string
	Err      error
}

// SearchFunc performs a search without blocking the Bubble Tea update loop.
type SearchFunc func(context.Context, string, string, int) Response

// OpenResultAction asks the parent model to open the selected result.
type OpenResultAction struct {
	Result Result
}

func (a OpenResultAction) String() string {
	return "OpenResultAction for " + a.Result.Path
}

// Model contains the global-search modal state.
type Model struct {
	open        bool
	root        string
	search      SearchFunc
	textInput   textinput.Model
	nextReqID   uint64
	activeReqID uint64
	cancel      context.CancelFunc
	loading     bool
	results     []Result
	provider    string
	partial     bool
	notice      string
	errText     string
	cursor      int
	renderIndex int
	width       int
	maxHeight   int
}

type debounceMsg struct {
	reqID uint64
	query string
}

type searchResultMsg struct {
	reqID    uint64
	query    string
	response Response
}
