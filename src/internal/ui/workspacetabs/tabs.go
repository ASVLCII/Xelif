// Package workspacetabs shares workspace-strip geometry between paint and input.
package workspacetabs

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const (
	NewWorkspace         = -1
	Picker               = -2
	compactControlsWidth = 44
	maximumTitleCells    = 26
)

type Tab struct{ ID, Title string }
type Cell struct {
	Index, Start, End int
	Text              string
	Active            bool
}
type Layout struct {
	Width int
	Cells []Cell
}

// Compute always retains the active workspace and both controls on overflow.
func Compute(tabs []Tab, active, width int) Layout {
	l := Layout{Width: max(width, 0)}
	if width <= 0 {
		return l
	}
	controls, create := " Workspaces ", " + New workspace "
	if width < compactControlsWidth {
		controls, create = " Tabs ", " + New "
	}
	budget := max(0, width-ansi.StringWidth(controls)-ansi.StringWidth(create))
	labels, total := make([]string, len(tabs)), 0
	for i, tab := range tabs {
		name := strings.Map(func(r rune) rune {
			if r < 32 || r == 127 {
				return -1
			}
			return r
		}, ansi.Strip(tab.Title))
		labels[i] = " " + ansi.Truncate(name, maximumTitleCells, "...") + " "
		total += ansi.StringWidth(labels[i])
	}
	x := 0
	add := func(index int, label string, active bool) {
		label = ansi.Truncate(label, max(0, width-x), "")
		end := x + ansi.StringWidth(label)
		if end > x {
			l.Cells = append(l.Cells, Cell{index, x, end, label, active})
		}
		x = end
	}
	if total <= budget {
		for i, label := range labels {
			add(i, label, i == active)
		}
	} else if active >= 0 && active < len(labels) {
		add(active, ansi.Truncate(labels[active], budget, "..."), true)
	}
	x = max(x, width-ansi.StringWidth(controls)-ansi.StringWidth(create))
	add(Picker, controls, false)
	add(NewWorkspace, create, false)
	return l
}

func (l Layout) Hit(x int) (int, bool) {
	for _, c := range l.Cells {
		if x >= c.Start && x < c.End {
			return c.Index, true
		}
	}
	return 0, false
}

func (l Layout) Render(style func(string, bool) string) string {
	var out strings.Builder
	x := 0
	for _, c := range l.Cells {
		out.WriteString(strings.Repeat(" ", max(0, c.Start-x)))
		out.WriteString(style(c.Text, c.Active))
		x = c.End
	}
	out.WriteString(strings.Repeat(" ", max(0, l.Width-x)))
	return out.String()
}
