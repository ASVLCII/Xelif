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
	for _, size := range [][2]int{{60, 24}, {80, 24}, {132, 40}, {180, 50}} {
		TeaUpdate(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		view := m.appearanceRender()
		m.firstUse, m.firstLoadingComplete = true, true
		require.LessOrEqual(t, lipgloss.Width(view), size[0])
		require.LessOrEqual(t, lipgloss.Height(view), size[1])
		require.Contains(t, view, "Portable mode")
		require.Contains(t, view, "Rich icons")
		require.Contains(t, view, "AND selected")
		require.Zero(t, m.appearanceChoice, "safe choice has first focus")
		frame := strings.Split(ansi.Strip(m.View().Content), "\n")
		x, y := size[0]/2-lipgloss.Width(view)/2, size[1]/2-lipgloss.Height(view)/2
		require.Equal(t, strings.Split(ansi.Strip(view), "\n")[0], ansi.Cut(frame[y], x, x+lipgloss.Width(view)))
		t.Logf("terminal %dx%d: centered welcome %dx%d", size[0], size[1], lipgloss.Width(view), lipgloss.Height(view))
	}
}
