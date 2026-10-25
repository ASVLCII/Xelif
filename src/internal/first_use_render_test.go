package internal

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestFirstUseFitsStandardTerminal(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	for _, size := range [][2]int{{80, 24}, {132, 40}, {180, 50}} {
		TeaUpdate(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		view := m.introduceModalRender()
		m.firstUse, m.firstLoadingComplete = true, true
		require.LessOrEqual(t, lipgloss.Width(view), 60, "welcome box should fit its text, not the terminal")
		require.LessOrEqual(t, lipgloss.Height(view), 20)
		require.Contains(t, view, "Inspired by github.com/yorukot/superfile.")
		require.NotContains(t, view, "Built from")
		require.Contains(t, view, "continue")
		require.Contains(t, ansi.Strip(view), "Click folders, breadcrumbs, tabs, Search, or Commands.")
		frame := strings.Split(ansi.Strip(m.View().Content), "\n")
		x, y := size[0]/2-lipgloss.Width(view)/2, size[1]/2-lipgloss.Height(view)/2
		require.Equal(t, strings.Split(ansi.Strip(view), "\n")[0], ansi.Cut(frame[y], x, x+lipgloss.Width(view)))
		t.Logf("terminal %dx%d: centered welcome %dx%d", size[0], size[1], lipgloss.Width(view), lipgloss.Height(view))
	}
}
