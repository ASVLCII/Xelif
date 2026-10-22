package internal

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/require"
)

func TestFirstUseFitsStandardTerminal(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	TeaUpdate(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	view := m.introduceModalRender()
	require.LessOrEqual(t, lipgloss.Width(view), 80)
	require.LessOrEqual(t, lipgloss.Height(view), 24)
	require.Contains(t, view, "continue")
}
