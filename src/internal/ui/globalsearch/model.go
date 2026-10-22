package globalsearch

import (
	"context"
	"errors"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/common"
)

const (
	searchDebounceDelay = 100 * time.Millisecond
	searchTimeout       = 2 * time.Second
	defaultResultLimit  = 100
)

// New creates a global-search modal with an injectable asynchronous search function.
func New(search SearchFunc) Model {
	if search == nil {
		search = NewDefaultSearchFunc()
	}
	m := Model{
		search:    search,
		textInput: common.GeneratePromptTextInput(),
	}
	m.SetWidth(MinWidth)
	m.SetMaxHeight(MinHeight)
	return m
}

// Open resets and focuses the modal for searches rooted at root.
func (m *Model) Open(root string) tea.Cmd {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
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

// Close closes and clears the modal.
func (m *Model) Close() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
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
	m.textInput.SetValue("")
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
	if msg.reqID != m.activeReqID || msg.query != m.textInput.Value() {
		return
	}
	m.loading = false
	m.cancel = nil
	m.results = append(m.results[:0], msg.response.Results...)
	m.provider = msg.response.Provider
	m.partial = msg.response.Partial
	m.notice = msg.response.Notice
	m.errText = ""
	if msg.response.Err != nil &&
		!errors.Is(msg.response.Err, context.Canceled) &&
		!errors.Is(msg.response.Err, context.DeadlineExceeded) {
		m.errText = msg.response.Err.Error()
	}
	m.cursor = 0
	m.renderIndex = 0
}

func (m *Model) beginSearch(msg debounceMsg) tea.Cmd {
	if msg.reqID != m.activeReqID || msg.query != m.textInput.Value() {
		return nil
	}
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
	m.cancel = cancel
	m.loading = true
	search := m.search
	root := m.root
	return func() tea.Msg {
		defer cancel()
		return searchResultMsg{
			reqID:    msg.reqID,
			query:    msg.query,
			response: search(ctx, msg.query, root, defaultResultLimit),
		}
	}
}

func (m *Model) handleKeyPress(msg tea.KeyPressMsg) (common.ModelAction, tea.Cmd) {
	action := common.ModelAction(common.NoAction{})
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
	m.nextReqID++
	m.activeReqID = m.nextReqID
	reqID := m.activeReqID
	return tea.Tick(searchDebounceDelay, func(time.Time) tea.Msg {
		return debounceMsg{reqID: reqID, query: query}
	})
}

func (m *Model) selectResult(result Result) common.ModelAction {
	if m.savedResults != nil {
		// The parent closes after a successful reveal. Missing paths leave the
		// picker and its filter intact so the next result remains reachable.
		return OpenResultAction{Result: result, Reveal: true}
	}
	m.Close()
	return OpenResultAction{Result: result}
}
