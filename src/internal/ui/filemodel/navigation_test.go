package filemodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/ui/filepanel"
)

func TestFocusPanelSelectsRequestedPanel(t *testing.T) {
	m := Model{
		FilePanels: []filepanel.Model{
			{IsFocused: true},
			{IsFocused: false},
			{IsFocused: false},
		},
		FocusedPanelIndex: 0,
	}

	require.True(t, m.FocusPanel(2))
	assert.Equal(t, 2, m.FocusedPanelIndex)
	assert.False(t, m.FilePanels[0].IsFocused)
	assert.False(t, m.FilePanels[1].IsFocused)
	assert.True(t, m.FilePanels[2].IsFocused)
}

func TestFocusPanelRejectsNegativeIndex(t *testing.T) {
	m := Model{
		FilePanels:        []filepanel.Model{{IsFocused: true}, {IsFocused: false}},
		FocusedPanelIndex: 0,
	}

	assert.False(t, m.FocusPanel(-1))
	assert.Equal(t, 0, m.FocusedPanelIndex)
	assert.True(t, m.FilePanels[0].IsFocused)
	assert.False(t, m.FilePanels[1].IsFocused)
}

func TestFocusPanelRejectsIndexPastLastPanel(t *testing.T) {
	m := Model{
		FilePanels:        []filepanel.Model{{IsFocused: true}, {IsFocused: false}},
		FocusedPanelIndex: 0,
	}

	assert.False(t, m.FocusPanel(2))
	assert.Equal(t, 0, m.FocusedPanelIndex)
	assert.True(t, m.FilePanels[0].IsFocused)
	assert.False(t, m.FilePanels[1].IsFocused)
}
