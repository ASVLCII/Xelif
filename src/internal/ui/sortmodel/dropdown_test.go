package sortmodel

import (
	"os"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestDropdownPreservesIndependentFieldAndDirection(t *testing.T) {
	m := New()
	m.Open(SortBySize, true)
	require.Equal(t, 3, m.Cursor)
	plain := ansi.Strip(m.Render())
	require.Equal(t, 10, lipgloss.Height(m.Render()), "separator must not wrap and shift direction hit targets")
	require.Contains(t, plain, "● Size")
	require.Contains(t, plain, "● Descending")
	require.Equal(t, 2, strings.Count(plain, "●"))
	m.Cursor = 5
	require.Equal(t, SortBySize, m.GetSelectedKind())
	require.False(t, m.GetSelectedReversed())
	m.Cursor = 0
	require.Equal(t, SortByName, m.GetSelectedKind())
	require.True(t, m.GetSelectedReversed())
	_, hit := m.ItemAtRow(5)
	require.False(t, hit)
}

func TestDropdownStyleHandlesTransparentThemeAndNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	require.NoError(t, os.Unsetenv("NO_COLOR"))
	m := New()
	m.Open(SortByName)
	require.NotPanics(t, func() { controlStyle(nil).Render("test") })
	t.Setenv("NO_COLOR", "1")
	require.Contains(t, ansi.Strip(m.Render()), ">● Name")
}
