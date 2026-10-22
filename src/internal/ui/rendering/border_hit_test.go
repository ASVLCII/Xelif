package rendering

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBorderInfoItemAtXMatchesPackedBottomBorderLayout(t *testing.T) {
	items := []string{"Sort", "Mode"}

	_, ok := BorderInfoItemAtX(20, items, 4)
	assert.False(t, ok, "left-side border filler must not be clickable")

	index, ok := BorderInfoItemAtX(20, items, 5)
	require.True(t, ok)
	assert.Equal(t, 0, index)

	index, ok = BorderInfoItemAtX(20, items, 12)
	require.True(t, ok)
	assert.Equal(t, 1, index)

	_, ok = BorderInfoItemAtX(20, items, 19)
	assert.False(t, ok, "outer right corner must not be clickable")
}
