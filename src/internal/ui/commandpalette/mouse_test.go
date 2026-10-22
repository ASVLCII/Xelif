package commandpalette

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
)

func TestHandleClickExecutesVisibleLocalRowAndCloses(t *testing.T) {
	t.Parallel()

	m := New([]Item{{ID: "zero", Label: "Zero"}, {ID: "one", Label: "One"}, {ID: "two", Label: "Two"}})
	_ = m.Open()
	m.renderIndex = 1

	action := m.HandleClick(firstItemRow + 1)

	require.Equal(t, ExecuteAction{ID: "two"}, action)
	assert.False(t, m.IsOpen())
}

func TestHandleClickIgnoresRowsOutsideVisibleItems(t *testing.T) {
	t.Parallel()

	for _, localY := range []int{-1, 0, firstItemRow - 1, firstItemRow + 2, 100} {
		m := New([]Item{{ID: "zero", Label: "Zero"}, {ID: "one", Label: "One"}})
		_ = m.Open()

		action := m.HandleClick(localY)

		require.IsType(t, common.NoAction{}, action, "local y=%d", localY)
		assert.True(t, m.IsOpen(), "local y=%d", localY)
	}
}

func TestHandleClickDoesNotExecuteDisabledItem(t *testing.T) {
	t.Parallel()

	m := New([]Item{{ID: "disabled", Label: "Disabled", Disabled: true}})
	_ = m.Open()

	action := m.HandleClick(firstItemRow)

	require.IsType(t, common.NoAction{}, action)
	assert.True(t, m.IsOpen())
}

func TestHandleClickOnClosedPaletteDoesNothing(t *testing.T) {
	t.Parallel()

	m := New([]Item{{ID: "item", Label: "Item"}})
	action := m.HandleClick(firstItemRow)

	require.IsType(t, common.NoAction{}, action)
}
