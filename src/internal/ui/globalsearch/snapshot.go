package globalsearch

import tea "charm.land/bubbletea/v2"

// SearchSnapshot is value-only workspace state. It owns its result slices and
// contains no contexts, functions, tasks, or mutable text input internals.
// Epochs and request counters are intentionally NOT restorable.
type SearchSnapshot struct {
	Open         bool
	Root         string
	Query        string
	Scope        Scope
	Results      []Result
	SavedResults []Result
	ResultTitle  string
	Provider     string
	Partial      bool
	Notice       string
	ErrorText    string
	Cursor       int
	RenderIndex  int
	Pending      bool
}

func (m *Model) Snapshot() SearchSnapshot {
	snapshot := SearchSnapshot{Open: m.open, Root: m.root, Query: m.GetQuery(), Scope: m.GetScope(),
		Results: append([]Result(nil), m.results...), ResultTitle: m.resultTitle,
		Provider: m.provider, Partial: m.partial, Notice: m.notice, ErrorText: m.errText,
		Cursor: m.cursor, RenderIndex: m.renderIndex, Pending: m.loading}
	if m.savedResults != nil {
		snapshot.SavedResults = make([]Result, len(m.savedResults))
		copy(snapshot.SavedResults, m.savedResults)
	}
	return snapshot
}

// Restore cancels the previous workspace's request before restoring values.
// A pending search gets a fresh epoch/generation; an old queued result cannot
// satisfy it even when query text and folder happen to be identical.
func (m *Model) Restore(snapshot SearchSnapshot) tea.Cmd {
	m.invalidate()
	m.open, m.root, m.scope = snapshot.Open, snapshot.Root, normalizeScope(snapshot.Scope)
	m.textInput.SetValue(snapshot.Query)
	m.results = append([]Result(nil), snapshot.Results...)
	m.savedResults = nil
	if snapshot.SavedResults != nil {
		m.savedResults = make([]Result, len(snapshot.SavedResults))
		copy(m.savedResults, snapshot.SavedResults)
	}
	m.resultTitle, m.provider, m.partial = snapshot.ResultTitle, snapshot.Provider, snapshot.Partial
	m.notice, m.errText = snapshot.Notice, snapshot.ErrorText
	m.cursor, m.renderIndex = snapshot.Cursor, snapshot.RenderIndex
	m.cursor = max(0, min(m.cursor, len(m.results)-1))
	m.renderIndex = max(0, min(m.renderIndex, m.cursor))
	m.loading = false
	if !m.open {
		m.textInput.Blur()
		return nil
	}
	_ = m.textInput.Focus()
	if snapshot.Pending && m.savedResults == nil {
		return m.scheduleSearch(snapshot.Query)
	}
	return nil
}

// Resume reopens the last query/result set after revealing a path. It does not
// reset the requested scope or start an unsolicited filesystem search.
func (m *Model) Resume() tea.Cmd {
	snapshot := m.Snapshot()
	snapshot.Open = true
	return m.Restore(snapshot)
}
