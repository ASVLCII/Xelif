package commandpalette

import (
	"strconv"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
)

func TestArrowNavigationWraps(t *testing.T) {
	t.Parallel()

	m := New(testItems())
	_ = m.Open()

	_, _ = m.HandleUpdate(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 2, m.cursor)
	_, _ = m.HandleUpdate(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Zero(t, m.cursor)
}

func TestArrowUpMovesToPreviousItem(t *testing.T) {
	t.Parallel()

	m := New(testItems())
	_ = m.Open()
	m.cursor = 2

	_, _ = m.HandleUpdate(tea.KeyPressMsg{Code: tea.KeyUp})

	assert.Equal(t, 1, m.cursor)
}

func TestNavigationKeepsCursorInBoundedWindow(t *testing.T) {
	t.Parallel()

	items := make([]Item, 6)
	for index := range items {
		indexText := strconv.Itoa(index)
		items[index] = Item{ID: indexText, Label: "Item " + indexText}
	}
	m := New(items)
	m.SetMaxHeight(MinHeight)
	_ = m.Open()

	_, _ = m.HandleUpdate(tea.KeyPressMsg{Code: tea.KeyDown})
	_, _ = m.HandleUpdate(tea.KeyPressMsg{Code: tea.KeyDown})

	assert.Equal(t, 2, m.cursor)
	assert.Equal(t, 1, m.renderIndex)
	assert.Equal(t, 2, m.visibleItemCount())
}

func TestEnterExecutesEnabledSelectionAndCloses(t *testing.T) {
	t.Parallel()

	m := New(testItems())
	_ = m.Open()
	_, _ = m.HandleUpdate(tea.KeyPressMsg{Code: tea.KeyDown})

	action, cmd := m.HandleUpdate(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Nil(t, cmd)
	require.Equal(t, ExecuteAction{ID: "branch"}, action)
	assert.False(t, m.IsOpen())
}

func TestEnterDoesNotExecuteOrCloseDisabledSelection(t *testing.T) {
	t.Parallel()

	m := New(testItems())
	_ = m.Open()
	m.cursor = 2

	action, cmd := m.HandleUpdate(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.IsType(t, common.NoAction{}, action)
	assert.Nil(t, cmd)
	assert.True(t, m.IsOpen())
}

func TestNavigationWithNoMatchesIsSafe(t *testing.T) {
	t.Parallel()

	m := New(nil)
	_ = m.Open()
	for _, key := range []rune{tea.KeyUp, tea.KeyDown, tea.KeyEnter} {
		action, cmd := m.HandleUpdate(tea.KeyPressMsg{Code: key})
		require.IsType(t, common.NoAction{}, action)
		assert.Nil(t, cmd)
	}
	assert.Zero(t, m.cursor)
	assert.Zero(t, m.renderIndex)
}
