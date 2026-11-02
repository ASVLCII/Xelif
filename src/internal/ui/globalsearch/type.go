package globalsearch

import (
	"context"

	"github.com/yorukot/superfile/src/internal/ui/processbar"

	"charm.land/bubbles/v2/textinput"
)

const (
	ProviderEverything = "Everything"
	ProviderSpotlight  = "Spotlight"
	ProviderPlocate    = "plocate"
	ProviderLocal      = "Folder"
)

// Scope is an explicit user choice, never an automatic provider fallback.
type Scope string

const (
	ScopeIndexed Scope = "indexed"
	ScopeFolder  Scope = "folder"
)

// ProviderIdentity names the backend independently of its display label.
type ProviderIdentity struct {
	ID   string
	Name string
}

// Capabilities declares which shared query features a provider can preserve.
// Indexed backends deliberately do not approximate advanced folder filters.
type Capabilities struct {
	NameTerms bool
	MultiTerm bool
	Filters   bool
	Content   bool
	Indexed   bool
}

// SearchRequest keeps scope, cancellation generation and the selected folder
// explicit. Indexed means the provider's existing index, not this Root.
type SearchRequest struct {
	Query      string
	Scope      Scope
	Root       string
	Limit      int
	Generation uint64
}

type ProviderSearchFunc func(context.Context, SearchRequest) Response

// Result is a file or directory returned by global search.
type Result struct {
	Path      string
	Directory bool
}

// Response contains one completed search request.
type Response struct {
	Results        []Result
	Provider       string
	Partial        bool
	Notice         string
	Err            error
	Identity       ProviderIdentity
	Capabilities   Capabilities
	RequestedScope Scope
	Root           string
	Generation     uint64
	Truncated      bool
}

// SearchFunc performs a search without blocking the Bubble Tea update loop.
type SearchFunc func(context.Context, string, string, int) Response

// OpenResultAction asks the parent model to open the selected result.
type OpenResultAction struct {
	Result Result
	Reveal bool
}

func (a OpenResultAction) String() string {
	return "OpenResultAction for " + a.Result.Path
}

// Model contains the global-search modal state.
type Model struct {
	Tasks          *processbar.Model
	open           bool
	root           string
	search         SearchFunc
	providerSearch ProviderSearchFunc
	scope          Scope
	epoch          uint64
	textInput      textinput.Model
	nextReqID      uint64
	activeReqID    uint64
	cancel         context.CancelFunc
	loading        bool
	results        []Result
	savedResults   []Result
	resultTitle    string
	provider       string
	partial        bool
	notice         string
	errText        string
	cursor         int
	renderIndex    int
	width          int
	maxHeight      int
}

type debounceMsg struct {
	reqID uint64
	query string
	epoch uint64
	scope Scope
	root  string
}

type searchResultMsg struct {
	reqID    uint64
	query    string
	response Response
	epoch    uint64
	scope    Scope
	root     string
}
