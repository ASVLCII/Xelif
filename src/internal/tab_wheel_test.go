package internal

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

func TestWheelOnTabStripCyclesWithoutMovingFileCursor(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	panel := m.getFocusedFilePanel()
	require.NoError(t, panel.NewTab(panel.Location))
	cursor := panel.GetCursor()
	m.focusPanel = sidebarFocus
	x := m.sidebarModel.GetWidth() + 1
	m.handleMouseMsg(tea.MouseWheelMsg{X: x, Y: 1, Button: tea.MouseWheelUp})
	require.Equal(t, 0, panel.ActiveTabIndex())
	require.Equal(t, cursor, panel.GetCursor())
	require.Equal(t, nonePanelFocus, m.focusPanel)
	m.handleMouseMsg(tea.MouseWheelMsg{X: x, Y: 1, Button: tea.MouseWheelDown})
	require.Equal(t, 1, panel.ActiveTabIndex())
}
