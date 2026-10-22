package globalsearch

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestSavedResultsFilterWithoutProviderAndPreserveSnapshot(t *testing.T) {
	t.Parallel()
	calls := 0
	m := New(func(context.Context, string, string, int) Response { calls++; return Response{} })
	snapshot := []Result{{Path: "/one/Report.TXT"}, {Path: "/two/image.png"}}
	m.OpenResults("Collection: work", snapshot)
	snapshot[0].Path = "changed by caller"
	assert.Contains(t, ansi.Strip(m.Render()), "Collection: work")
	assert.Len(t, m.GetResults(), 2)
	_, cmd := m.HandleUpdate(utils.TeaRuneKeyMsg("REPORT"))
	assert.Zero(t, calls)
	if cmd != nil {
		// The input cursor's blink command is unrelated to a search request.
		assert.False(t, m.IsLoading())
	}
	require.Len(t, m.GetResults(), 1)
	assert.Equal(t, "/one/Report.TXT", m.GetResults()[0].Path)
	action, _ := m.HandleUpdate(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.IsType(t, OpenResultAction{}, action)
	assert.True(t, action.(OpenResultAction).Reveal)
	assert.True(t, m.IsOpen(), "parent decides whether the reveal succeeded")
	m.textInput.SetValue("")
	assert.Nil(t, m.scheduleSearch(""))
	assert.Len(t, m.GetResults(), 2)
	assert.Zero(t, calls)
	_ = m.Open("/normal")
	assert.Nil(t, m.savedResults)
	assert.Empty(t, m.resultTitle)
}

func TestSavedResultsIgnorePriorSearchReplies(t *testing.T) {
	t.Parallel()
	calls, cancellations := 0, 0
	m := New(func(context.Context, string, string, int) Response { calls++; return Response{} })
	const query = "old"
	_ = m.OpenQuery("/root", query)
	oldID := m.activeReqID
	m.cancel = func() { cancellations++ }
	snapshot := []Result{{Path: "/saved-old.txt"}, {Path: "/other.txt"}}
	m.OpenResults("Tag: work", snapshot)
	require.Equal(t, 1, cancellations, "opening saved results must cancel the prior search")
	m.HandleUpdate(utils.TeaRuneKeyMsg(query))
	require.Equal(t, query, m.GetQuery(), "the stale and current queries must match")
	want := []Result{{Path: "/saved-old.txt"}}
	require.Equal(t, want, m.GetResults())

	// Matching query text cannot authorize either an old completion or an old
	// debounce tick to replace or search outside the saved snapshot.
	m.HandleUpdate(searchResultMsg{
		reqID: oldID, query: query,
		response: Response{Results: []Result{{Path: "/stale"}}, Provider: ProviderEverything},
	})
	_, cmd := m.HandleUpdate(debounceMsg{reqID: oldID, query: query})
	assert.Nil(t, cmd, "a queued live-search debounce must not start a provider")
	assert.Equal(t, want, m.GetResults())
	assert.Equal(t, snapshot, m.savedResults)
	assert.Empty(t, m.GetProvider())
	assert.False(t, m.IsLoading())
	assert.Nil(t, m.cancel)
	assert.Zero(t, calls)
	assert.Equal(t, 1, cancellations, "stale messages must not cancel or replace picker state")
}

func TestPastedSavedFilterChangesSelection(t *testing.T) {
	t.Parallel()
	m := New(func(context.Context, string, string, int) Response {
		t.Fatal("saved picker must never search outside its snapshot")
		return Response{}
	})
	m.OpenResults("Collection", []Result{{Path: "/one.txt"}, {Path: "/two.png"}})
	m.HandleUpdate(tea.PasteMsg{Content: "png"})
	require.Equal(t, []Result{{Path: "/two.png"}}, m.GetResults())
	action, _ := m.HandleUpdate(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, "/two.png", action.(OpenResultAction).Result.Path)
}

func TestEmptySavedResultsRenderAsResultsNotSearchPrompt(t *testing.T) {
	t.Parallel()
	m := New(nil)
	m.SetWidth(90)
	m.SetMaxHeight(20)
	m.OpenResults("Collection: empty", nil)
	assert.NotNil(t, m.savedResults)
	assert.NotContains(t, ansi.Strip(m.Render()), "Type to search")
	assert.Empty(t, m.GetResults())
}

func TestClosingSavedResultsReleasesBuffers(t *testing.T) {
	t.Parallel()
	m := New(nil)
	m.OpenResults("Collection", []Result{{Path: "/saved"}})
	m.HandleUpdate(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, m.IsOpen())
	assert.Nil(t, m.savedResults)
	assert.Nil(t, m.results)
	assert.Empty(t, m.resultTitle)
	// Preserve the pre-existing Close contract for ordinary live searches.
	_ = m.Open("/root")
	m.results = []Result{{Path: "/live"}}
	m.Close()
	assert.Equal(t, []Result{{Path: "/live"}}, m.results)
}
