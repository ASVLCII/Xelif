package filepanel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSearchActionsRemainClickableInMinimumWidthPanel(t *testing.T) {
	panel := Model{width: MinWidth}
	search, commands := panel.SearchActionAtContentX(0)
	assert.True(t, search)
	assert.False(t, commands)
	search, commands = panel.SearchActionAtContentX(8)
	assert.False(t, search)
	assert.True(t, commands)
	search, commands = panel.SearchActionAtContentX(panel.GetContentWidth())
	assert.False(t, search)
	assert.False(t, commands)
}
