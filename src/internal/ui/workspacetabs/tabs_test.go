package workspacetabs

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestGeometryAndOverflow(t *testing.T) {
	tabs := []Tab{{"a", "Downloads"}, {"b", "資料 e\u0301"}, {"c", strings.Repeat("long", 30)}}
	for _, width := range []int{60, 80, 120, 180} {
		for active := range tabs {
			l := Compute(tabs, active, width)
			seen := map[int]bool{}
			for _, c := range l.Cells {
				i, ok := l.Hit(c.Start)
				require.True(t, ok)
				require.Equal(t, c.Index, i)
				seen[c.Index] = true
				require.LessOrEqual(t, c.End, width)
			}
			require.True(t, seen[active])
			require.True(t, seen[Picker])
			require.True(t, seen[NewWorkspace])
			require.Equal(t, width, ansi.StringWidth(l.Render(func(s string, _ bool) string { return s })))
		}
	}
}

func TestRejectsControlInjection(t *testing.T) {
	l := Compute([]Tab{{"a", "bad\n\x1b[31mred\x1b[0m"}}, 0, 80)
	view := l.Render(func(s string, _ bool) string { return s })
	require.NotContains(t, view, "\n")
	require.NotContains(t, view, "\x1b")
}
