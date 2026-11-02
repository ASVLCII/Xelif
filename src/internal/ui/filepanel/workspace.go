package filepanel

import (
	"maps"
	"os"

	"github.com/yorukot/superfile/src/internal/session"
	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

// ViewState is in-memory only. It contains no workers, input models or render caches.
// Selection identities prevent a replaced path from silently becoming selected.
type ViewState struct {
	Cursor, Offset      int
	FocusedPath, Filter string
	FilterFocused       bool
	Mode                PanelMode
	Selected            map[string]os.FileInfo
	SelectionOrder      map[string]int
	SelectionCounter    int
	SelectionAnchor     string
	Records             map[string]directoryRecord
}

func (m *Model) SnapshotView() ViewState {
	v := ViewState{
		Cursor:           m.cursor,
		Offset:           m.renderIndex,
		Filter:           m.SearchBar.Value(),
		FilterFocused:    m.SearchBar.Focused(),
		Mode:             m.PanelMode,
		FocusedPath:      m.GetFocusedItem().Location,
		Records:          maps.Clone(m.DirectoryRecords),
		Selected:         make(map[string]os.FileInfo),
		SelectionOrder:   maps.Clone(m.selected),
		SelectionCounter: m.selectOrderCounter,
		SelectionAnchor:  m.selectionAnchor,
	}
	for _, e := range m.element {
		if !m.CheckSelected(e.Location) {
			continue
		}
		// A visible selection with unavailable identity must not be recaptured
		// from a replacement path by the hidden-selection fallback below.
		v.Selected[e.Location] = nil
		// On Windows file IDs may be loaded lazily from the path. Force that
		// lookup now, before a replacement can appear while we are inactive.
		if e.Info != nil && os.SameFile(e.Info, e.Info) {
			v.Selected[e.Location] = e.Info
		}
	}
	// Filtering does not clear selection. Preserve selected paths outside the
	// visible rows too, without scanning or fetching metadata for unrelated files.
	for path := range m.selected {
		if _, visible := v.Selected[path]; visible {
			continue
		}
		if info, err := os.Stat(path); err == nil && os.SameFile(info, info) {
			v.Selected[path] = info
		}
	}
	return v
}

func (m *Model) RestoreView(v ViewState, dotFiles bool) {
	m.DirectoryRecords = maps.Clone(v.Records)
	if m.DirectoryRecords == nil {
		m.DirectoryRecords = make(map[string]directoryRecord)
	}
	m.SearchBar.SetValue(v.Filter)
	m.UpdateElementsIfNeeded(true, dotFiles)
	m.PanelMode = v.Mode
	m.ResetSelected()
	for path, old := range v.Selected {
		if old == nil {
			continue
		}
		if current, err := os.Stat(path); err == nil && os.SameFile(old, current) {
			m.selected[path] = v.SelectionOrder[path]
		}
	}
	m.selectOrderCounter = v.SelectionCounter
	if m.CheckSelected(v.SelectionAnchor) {
		m.selectionAnchor = v.SelectionAnchor
	}
	m.cursor = min(max(v.Cursor, 0), max(0, m.ElemCount()-1))
	if i := m.FindElementIndexByLocation(v.FocusedPath); i >= 0 {
		m.cursor = i
	}
	m.renderIndex = min(max(v.Offset, 0), m.cursor)
	m.scrollToCursor(m.cursor)
	if v.FilterFocused {
		m.SearchBar.Focus()
	} else {
		m.SearchBar.Blur()
	}
}

// RestoreWorkspace retains offline locations and histories. Reading an unavailable
// directory may show an empty panel, but must not rewrite its saved navigation.
func RestoreWorkspace(state session.PanelState, fallback string, focused bool) Model {
	if len(state.Tabs) == 0 {
		return New(fallback, focused, "", sortmodel.SortByName, false)
	}
	active := min(max(state.ActiveTab, 0), len(state.Tabs)-1)
	kind := sortmodel.SortKind(state.SortKind)
	if kind < sortmodel.SortByName || kind > sortmodel.SortByNatural {
		kind = sortmodel.SortByName
	}
	m := New(state.Tabs[active].Location, focused, "", kind, state.SortReversed)
	m.tabs = make([]panelTab, len(state.Tabs))
	for i, tab := range state.Tabs {
		m.tabs[i] = panelTab{
			location:       tab.Location,
			backHistory:    append([]string(nil), tab.BackHistory...),
			forwardHistory: append([]string(nil), tab.ForwardHistory...),
		}
	}
	m.activeTab = active
	return m
}
