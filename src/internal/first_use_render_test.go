package internal

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/require"
)

func TestFirstUseFitsStandardTerminal(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	for _, size := range [][2]int{{80, 24}, {132, 40}, {180, 50}} {
		TeaUpdate(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		view := m.introduceModalRender()
		require.LessOrEqual(t, lipgloss.Width(view), 60, "welcome box should fit its text, not the terminal")
		require.LessOrEqual(t, lipgloss.Height(view), 20)
		require.Contains(t, view, "Inspired by github.com/yorukot/superfile.")
		require.NotContains(t, view, "Built from")
		require.Contains(t, view, "continue")
	}
}
