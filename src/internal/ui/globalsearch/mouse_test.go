package globalsearch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
)

func TestHandleClickOpensVisibleResultAndCloses(t *testing.T) {
	t.Parallel()

	m := New(func(context.Context, string, string, int) Response { return Response{} })
	m.Open(t.TempDir())
	m.results = []Result{{Path: "zero"}, {Path: "one"}, {Path: "two"}}
	m.renderIndex = 1
	want := m.results[2]

	action := m.HandleClick(4)

	openAction, ok := action.(OpenResultAction)
	require.True(t, ok, "result click should return OpenResultAction, got %T", action)
	assert.Equal(t, want, openAction.Result)
	assert.False(t, m.IsOpen())
}

func TestHandleClickIgnoresNonResultRows(t *testing.T) {
	t.Parallel()

	for _, localY := range []int{-1, 0, 1, 2, 5, 100} {
		m := New(func(context.Context, string, string, int) Response { return Response{} })
		m.Open(t.TempDir())
		m.results = []Result{{Path: "zero"}, {Path: "one"}}

		action := m.HandleClick(localY)

		require.IsType(t, common.NoAction{}, action, "local y=%d", localY)
		assert.True(t, m.IsOpen(), "local y=%d", localY)
	}
}
