package globalsearch

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestModelOpenAndClose(t *testing.T) {
	t.Parallel()

	search := func(context.Context, string, string, int) Response { return Response{} }
	m := New(search)

	cmd := m.Open(`C:\Work`)

	assert.Nil(t, cmd)
	assert.True(t, m.IsOpen())
	assert.Equal(t, `C:\Work`, m.GetRoot())

	m.Close()
	assert.False(t, m.IsOpen())
}

func TestOpenClearsPreviousSearchState(t *testing.T) {
	t.Parallel()

	m := New(func(context.Context, string, string, int) Response { return Response{} })
	m.results = []Result{{Path: "old"}}
	m.provider = ProviderEverything
	m.notice = "old notice"
	m.errText = "old error"
	m.partial = true
	m.cursor = 2
	m.renderIndex = 1
	m.loading = true

	_ = m.Open(t.TempDir())

	assert.Empty(t, m.GetResults())
	assert.Empty(t, m.GetProvider())
	assert.Empty(t, m.notice)
	assert.Empty(t, m.errText)
	assert.False(t, m.partial)
	assert.False(t, m.IsLoading())
	assert.Zero(t, m.cursor)
	assert.Zero(t, m.renderIndex)
}

func TestNewUsesDefaultSearchWhenNoneInjected(t *testing.T) {
	t.Parallel()

	m := New(nil)

	assert.NotNil(t, m.search)
}

func TestOpenDoesNotSuppressFirstPrintableKey(t *testing.T) {
	t.Parallel()

	m := New(func(context.Context, string, string, int) Response { return Response{} })
	_ = m.Open(t.TempDir())

	_, _ = m.HandleUpdate(utils.TeaRuneKeyMsg("g"))
	assert.Equal(t, "g", m.GetQuery())
}

func TestHandleUpdateSchedulesSearchWhenQueryChanges(t *testing.T) {
	t.Parallel()

	search := func(context.Context, string, string, int) Response { return Response{} }
	m := New(search)
	m.Open(t.TempDir())

	action, cmd := m.HandleUpdate(utils.TeaRuneKeyMsg("a"))

	require.IsType(t, common.NoAction{}, action)
	assert.Equal(t, "a", m.GetQuery())
	assert.NotNil(t, cmd)
}

func TestDebounceStartsBoundedAsyncSearch(t *testing.T) {
	t.Parallel()

	search := func(ctx context.Context, query string, root string, limit int) Response {
		_, hasDeadline := ctx.Deadline()
		assert.True(t, hasDeadline)
		assert.Equal(t, "needle", query)
		assert.Equal(t, `C:\Work`, root)
		assert.Equal(t, defaultResultLimit, limit)
		return Response{Provider: ProviderEverything, Results: []Result{{Path: `C:\needle.txt`}}}
	}
	m := New(search)
	m.Open(`C:\Work`)
	m.activeReqID = 7
	m.textInput.SetValue("needle")

	action, cmd := m.HandleUpdate(debounceMsg{reqID: 7, query: "needle"})

	require.IsType(t, common.NoAction{}, action)
	require.NotNil(t, cmd)
	msg := cmd()
	resultMsg, ok := msg.(searchResultMsg)
	require.True(t, ok, "search command should return searchResultMsg, got %T", msg)
	assert.Equal(t, uint64(7), resultMsg.reqID)
	assert.Equal(t, "needle", resultMsg.query)
	assert.Equal(t, ProviderEverything, resultMsg.response.Provider)
}

func TestQueryChangeCancelsRunningSearchImmediately(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	canceled := make(chan struct{})
	search := func(ctx context.Context, _ string, _ string, _ int) Response {
		close(started)
		<-ctx.Done()
		close(canceled)
		return Response{Err: ctx.Err()}
	}
	m := New(search)
	m.Open(t.TempDir())
	m.textInput.SetValue("a")
	m.activeReqID = 1
	_, searchCmd := m.HandleUpdate(debounceMsg{reqID: 1, query: "a"})
	require.NotNil(t, searchCmd)
	defer func() {
		if m.cancel != nil {
			m.cancel()
		}
	}()
	go searchCmd()
	<-started

	_, _ = m.HandleUpdate(utils.TeaRuneKeyMsg("b"))

	select {
	case <-canceled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("running search was not canceled when the query changed")
	}
}

func TestCloseCancelsRunningSearch(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	canceled := make(chan struct{})
	search := func(ctx context.Context, _ string, _ string, _ int) Response {
		close(started)
		<-ctx.Done()
		close(canceled)
		return Response{Err: ctx.Err()}
	}
	m := New(search)
	m.Open(t.TempDir())
	m.textInput.SetValue("a")
	m.activeReqID = 1
	_, searchCmd := m.HandleUpdate(debounceMsg{reqID: 1, query: "a"})
	require.NotNil(t, searchCmd)
	defer func() {
		if m.cancel != nil {
			m.cancel()
		}
	}()
	go searchCmd()
	<-started

	m.Close()

	select {
	case <-canceled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("running search was not canceled when the modal closed")
	}
}

func TestCurrentSearchResultUpdatesModal(t *testing.T) {
	t.Parallel()

	m := New(func(context.Context, string, string, int) Response { return Response{} })
	m.Open(t.TempDir())
	m.textInput.SetValue("needle")
	m.activeReqID = 3
	m.loading = true
	want := []Result{{Path: `C:\needle.txt`}}

	action, cmd := m.HandleUpdate(searchResultMsg{
		reqID: 3,
		query: "needle",
		response: Response{
			Results:  want,
			Provider: ProviderEverything,
		},
	})

	require.IsType(t, common.NoAction{}, action)
	assert.Nil(t, cmd)
	assert.Equal(t, want, m.GetResults())
	assert.Equal(t, ProviderEverything, m.GetProvider())
	assert.False(t, m.IsLoading())
}

func TestRepeatedQueryRejectsOlderRequestID(t *testing.T) {
	t.Parallel()

	m := New(func(context.Context, string, string, int) Response { return Response{} })
	m.Open(t.TempDir())
	m.textInput.SetValue("A")
	m.activeReqID = 3
	m.results = []Result{{Path: "new-A"}}
	m.provider = ProviderLocal

	_, _ = m.HandleUpdate(searchResultMsg{
		reqID: 1,
		query: "A",
		response: Response{
			Results:  []Result{{Path: "old-A"}},
			Provider: ProviderEverything,
		},
	})

	assert.Equal(t, []Result{{Path: "new-A"}}, m.GetResults())
	assert.Equal(t, ProviderLocal, m.GetProvider())
}

func TestClearingQueryCancelsAndClearsResultsWithoutNewRequest(t *testing.T) {
	t.Parallel()

	m := New(func(context.Context, string, string, int) Response { return Response{} })
	m.Open(t.TempDir())
	m.textInput.SetValue("a")
	m.activeReqID = 4
	m.results = []Result{{Path: "old"}}
	m.provider = ProviderEverything
	m.loading = true

	_, _ = m.HandleUpdate(tea.KeyPressMsg{Code: tea.KeyBackspace})

	assert.Empty(t, m.GetQuery())
	assert.Empty(t, m.GetResults())
	assert.Empty(t, m.GetProvider())
	assert.False(t, m.IsLoading())
	assert.Equal(t, uint64(0), m.activeReqID)
}

func TestArrowNavigationWrapsResults(t *testing.T) {
	t.Parallel()

	m := New(func(context.Context, string, string, int) Response { return Response{} })
	m.Open(t.TempDir())
	m.results = []Result{{Path: "one"}, {Path: "two"}}

	_, _ = m.HandleUpdate(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 1, m.cursor)

	_, _ = m.HandleUpdate(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 0, m.cursor)
}

func TestNavigationKeepsCursorInsideVisibleWindow(t *testing.T) {
	t.Parallel()

	m := New(func(context.Context, string, string, int) Response { return Response{} })
	m.SetMaxHeight(MinHeight)
	m.results = []Result{{Path: "0"}, {Path: "1"}, {Path: "2"}, {Path: "3"}}
	m.cursor = 1

	m.navigateDown()

	assert.Equal(t, 2, m.cursor)
	assert.Equal(t, 1, m.renderIndex)
}

func TestEnterReturnsSelectedResultActionAndCloses(t *testing.T) {
	t.Parallel()

	m := New(func(context.Context, string, string, int) Response { return Response{} })
	m.Open(t.TempDir())
	m.results = []Result{
		{Path: `C:\one.txt`},
		{Path: `C:\Folder`, Directory: true},
	}
	m.cursor = 1
	want := m.results[1]

	action, cmd := m.HandleUpdate(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Nil(t, cmd)
	openAction, ok := action.(OpenResultAction)
	require.True(t, ok, "Enter should return OpenResultAction, got %T", action)
	assert.Equal(t, want, openAction.Result)
	assert.False(t, m.IsOpen())
}

func TestEscapeClosesWithoutAction(t *testing.T) {
	t.Parallel()

	m := New(func(context.Context, string, string, int) Response { return Response{} })
	m.Open(t.TempDir())

	action, cmd := m.HandleUpdate(tea.KeyPressMsg{Code: tea.KeyEscape})

	require.IsType(t, common.NoAction{}, action)
	assert.Nil(t, cmd)
	assert.False(t, m.IsOpen())
}
