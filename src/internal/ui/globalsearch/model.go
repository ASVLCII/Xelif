package globalsearch

import (
	"context"
	"errors"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/processbar"
)

const (
	searchDebounceDelay = 75 * time.Millisecond
	searchTimeout       = 2 * time.Second
	defaultResultLimit  = 100
)

// New creates a global-search modal with an injectable asynchronous search function.
func New(search SearchFunc) Model {
	var providerSearch ProviderSearchFunc
	if search == nil {
		search = NewDefaultSearchFunc()
		providerSearch = NewProviderSearchFunc()
	}
	m := Model{
		search:         search,
		providerSearch: providerSearch,
		scope:          ScopeIndexed,
		textInput:      common.GeneratePromptTextInput(),
	}
	m.SetWidth(MinWidth)
	m.SetMaxHeight(MinHeight)
	return m
}

// NewWithProvider supports explicit-scope providers without requiring callers
// to adapt them to the historical unscoped SearchFunc.
func NewWithProvider(search ProviderSearchFunc) Model {
	m := New(nil)
	if search != nil {
		m.providerSearch = search
	}
	return m
}

// Open resets and focuses the modal for searches rooted at root.
func (m *Model) Open(root string) tea.Cmd {
	m.invalidate()
	m.open = true
	m.root = root
	m.loading = false
	m.activeReqID = 0
	m.results = nil
	m.savedResults = nil
	m.resultTitle = ""
	m.provider = ""
	m.partial = false
	m.notice = ""
	m.errText = ""
	m.cursor = 0
	m.renderIndex = 0
	m.textInput.SetValue("")
	_ = m.textInput.Focus()
	return nil
}

// OpenQuery opens the modal with a caller-supplied query and immediately
// schedules the same debounced, cancellable search used by typed input.
func (m *Model) OpenQuery(root, query string) tea.Cmd {
	_ = m.Open(root)
	m.textInput.SetValue(query)
	return m.scheduleSearch(query)
}

// OpenResults reuses the search picker for a fixed set of saved paths. Filtering
// stays in memory and never falls through to a filesystem or Everything search.
func (m *Model) OpenResults(title string, results []Result) {
	_ = m.Open("")
	m.resultTitle = title
	m.savedResults = make([]Result, len(results))
	copy(m.savedResults, results)
	m.results = append([]Result(nil), results...)
	m.notice = "Enter or click to reveal · Esc to return"
}

// Close cancels work and invalidates all queued messages. Live query/results
// remain available to Resume and workspace snapshots; saved pickers release
// their potentially large private buffers.
func (m *Model) Close() {
	m.invalidate()
	m.open = false
	m.loading = false
	m.activeReqID = 0
	if m.savedResults != nil {
		// Collections can be much larger than a capped live-search response.
		// Release their extra snapshot/filter buffers when the picker closes.
		m.savedResults = nil
		m.results = nil
		m.resultTitle = ""
	}
	m.textInput.Blur()
}

func (m *Model) invalidate() {
	m.epoch++
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.activeReqID = 0
}

func (m *Model) IsOpen() bool {
	return m.open
}

func (m *Model) GetRoot() string {
	return m.root
}

func (m *Model) GetQuery() string {
	return m.textInput.Value()
}

func (m *Model) GetResults() []Result {
	return append([]Result(nil), m.results...)
}

func (m *Model) GetProvider() string {
	return m.provider
}

func (m *Model) GetScope() Scope { return normalizeScope(m.scope) }

// SetScope explicitly switches between the existing system index and a
// bounded search below GetRoot. The query is kept, never silently rewritten.
func (m *Model) SetScope(scope Scope) tea.Cmd {
	scope = normalizeScope(scope)
	if m.savedResults != nil || scope == m.GetScope() {
		return nil
	}
	m.invalidate()
	m.scope = scope
	if !m.open {
		return nil
	}
	return m.scheduleSearch(m.GetQuery())
}

// IsAsyncMessage lets the parent consume search events even when a picker or
// workspace is inactive. HandleUpdate still validates the modal's own epoch.
func IsAsyncMessage(msg tea.Msg) bool {
	switch msg.(type) {
	case debounceMsg, searchResultMsg:
		return true
	default:
		return false
	}
}

func (m *Model) IsLoading() bool {
	return m.loading
}

// HandleUpdate processes modal input and internal asynchronous messages.
func (m *Model) HandleUpdate(msg tea.Msg) (common.ModelAction, tea.Cmd) {
	action := common.ModelAction(common.NoAction{})
	if !m.open {
		return action, nil
	}

	switch msg := msg.(type) {
	case searchResultMsg:
		m.acceptSearchResult(msg)
		return action, nil
	case debounceMsg:
		return action, m.beginSearch(msg)
	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)
	default:
		oldQuery := m.textInput.Value()
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		if query := m.textInput.Value(); query != oldQuery {
			return action, tea.Batch(cmd, m.scheduleSearch(query))
		}
		return action, cmd
	}
}

// ShowResultError preserves the picker and filter when a saved path is missing.
func (m *Model) ShowResultError(message string) {
	m.notice = common.MakePrintableWithEscCheck(message, false)
}

func (m *Model) acceptSearchResult(msg searchResultMsg) {
	if !m.matchesRequest(msg.epoch, msg.reqID, msg.query, msg.scope, msg.root) {
		return
	}
	m.loading = false
	m.cancel = nil
	results := msg.response.Results
	if len(results) > defaultResultLimit {
		results = results[:defaultResultLimit]
		msg.response.Partial = true
		msg.response.Notice = appendNotice(msg.response.Notice, "Result limit reached; refine your search")
	}
	m.results = append(m.results[:0], results...)
	m.provider = msg.response.Provider
	m.partial = msg.response.Partial
	m.notice = common.MakePrintableWithEscCheck(msg.response.Notice, false)
	m.errText = ""
	if msg.response.Err != nil &&
		!errors.Is(msg.response.Err, context.Canceled) &&
		!errors.Is(msg.response.Err, context.DeadlineExceeded) {
		m.errText = common.MakePrintableWithEscCheck(msg.response.Err.Error(), false)
	}
	m.cursor = 0
	m.renderIndex = 0
}

func (m *Model) beginSearch(msg debounceMsg) tea.Cmd {
	if !m.matchesRequest(msg.epoch, msg.reqID, msg.query, msg.scope, msg.root) {
		return nil
	}
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
	m.cancel = cancel
	m.loading = true
	search := m.search
	providerSearch := m.providerSearch
	root := msg.root
	request := SearchRequest{Query: msg.query, Scope: msg.scope, Root: root,
		Limit: defaultResultLimit, Generation: msg.reqID}
	cmd := func() tea.Msg {
		defer cancel()
		var response Response
		switch {
		case providerSearch != nil:
			response = providerSearch(ctx, request)
		case request.Scope == ScopeFolder:
			parsed, err := ParseQuery(request.Query)
			if err != nil {
				response = Response{Provider: ProviderLocal, Err: err}
			} else {
				response = SearchLocalQuery(ctx, root, parsed, LocalSearchOptions{ResultLimit: defaultResultLimit})
			}
		default:
			response = search(ctx, msg.query, root, defaultResultLimit)
		}
		response.RequestedScope, response.Root, response.Generation = request.Scope, root, request.Generation
		return searchResultMsg{
			reqID: msg.reqID,
			query: msg.query,
			epoch: msg.epoch, scope: msg.scope, root: root,
			response: response,
		}
	}
	if m.Tasks == nil {
		return cmd
	}
	return m.Tasks.Track(
		processbar.TaskSpec{Kind: "Search", Source: root, Background: true, GroupKey: "global-search"},
		ctx,
		cancel,
		cmd,
		func(result tea.Msg) error {
			msg, _ := result.(searchResultMsg)
			return msg.response.Err
		},
	)
}

func (m *Model) matchesRequest(epoch, generation uint64, query string, scope Scope, root string) bool {
	return m.open && m.savedResults == nil && generation != 0 && epoch == m.epoch &&
		generation == m.activeReqID && query == m.GetQuery() && scope == m.GetScope() && root == m.root
}

func (m *Model) handleKeyPress(msg tea.KeyPressMsg) (common.ModelAction, tea.Cmd) {
	action := common.ModelAction(common.NoAction{})
	if msg.String() == "ctrl+s" && m.savedResults == nil {
		scope := ScopeFolder
		if m.GetScope() == ScopeFolder {
			scope = ScopeIndexed
		}
		return action, m.SetScope(scope)
	}
	switch msg.Code {
	case tea.KeyEscape:
		m.Close()
		return action, nil
	case tea.KeyEnter:
		if m.cursor >= 0 && m.cursor < len(m.results) {
			selected := m.results[m.cursor]
			return m.selectResult(selected), nil
		}
		return action, nil
	case tea.KeyUp:
		m.navigateUp()
		return action, nil
	case tea.KeyDown:
		m.navigateDown()
		return action, nil
	case tea.KeyPgUp:
		m.Scroll(-m.visibleResultCount())
		return action, nil
	case tea.KeyPgDown:
		m.Scroll(m.visibleResultCount())
		return action, nil
	}

	oldQuery := m.textInput.Value()
	var inputCmd tea.Cmd
	m.textInput, inputCmd = m.textInput.Update(msg)
	query := m.textInput.Value()
	if query == oldQuery {
		return action, inputCmd
	}
	return action, tea.Batch(inputCmd, m.scheduleSearch(query))
}

func (m *Model) scheduleSearch(query string) tea.Cmd {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	if m.savedResults != nil {
		m.results = m.results[:0]
		terms := strings.Fields(strings.ToLower(query))
		for _, result := range m.savedResults {
			path := strings.ToLower(result.Path)
			matches := true
			for _, term := range terms {
				if !strings.Contains(path, term) {
					matches = false
					break
				}
			}
			if matches {
				m.results = append(m.results, result)
			}
		}
		m.cursor, m.renderIndex = 0, 0
		return nil
	}
	if strings.TrimSpace(query) == "" {
		m.activeReqID = 0
		m.loading = false
		m.results = nil
		m.provider = ""
		m.partial = false
		m.notice = ""
		m.errText = ""
		m.cursor = 0
		m.renderIndex = 0
		return nil
	}
	m.results = nil
	m.provider = ""
	m.partial = false
	m.notice, m.errText = "", ""
	m.cursor, m.renderIndex = 0, 0
	m.loading = true
	m.nextReqID++
	m.activeReqID = m.nextReqID
	reqID := m.activeReqID
	epoch, scope, root := m.epoch, m.GetScope(), m.root
	return tea.Tick(searchDebounceDelay, func(time.Time) tea.Msg {
		return debounceMsg{reqID: reqID, query: query, epoch: epoch, scope: scope, root: root}
	})
}

func (m *Model) selectResult(result Result) common.ModelAction {
	if m.savedResults != nil {
		// The parent closes after a successful reveal. Missing paths leave the
		// picker and its filter intact so the next result remains reachable.
		return OpenResultAction{Result: result, Reveal: true}
	}
	// Resolve only the selected path in the parent reveal operation. Native
	// indexed results need no eager metadata requests for every hit. A failed
	// reveal leaves the search query/results intact.
	return OpenResultAction{Result: result, Reveal: true}
}
