package internal

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
)

func TestContextMenuLabelsRetainOneRowAndFitReferenceSizes(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	for _, size := range [][2]int{{60, 24}, {80, 24}, {120, 36}, {180, 50}} {
		m.fullWidth, m.fullHeight = size[0], size[1]
		for _, workspace := range []bool{false, true} {
			m.contextMenu = contextMenuState{open: true, workspaceTarget: workspace}
			m.positionContextMenu(tea.Mouse{X: size[0] - 1, Y: size[1] - 1})
			view := m.contextMenuRender()
			require.Equal(t, len(m.contextMenuLabels())+common.BorderPadding, lipgloss.Height(view))
			require.LessOrEqual(t, m.contextMenu.x+lipgloss.Width(view), size[0])
			require.LessOrEqual(t, m.contextMenu.y+lipgloss.Height(view), size[1])
			for _, label := range m.contextMenuLabels() {
				require.Contains(t, ansi.Strip(view), label)
			}
		}
	}
}

func TestContextMenuLastWorkspaceActionRemainsClickable(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	_ = m.newWorkspace(t.TempDir())
	_ = m.closeWorkspace()
	require.Len(t, m.workspaces, 1)
	m.openWorkspaceContextMenu(tea.Mouse{X: m.fullWidth - 1, Y: m.fullHeight - 1}, m.workspaces[0].State.ID)
	lastRow := m.contextMenu.y + len(m.contextMenuLabels())
	cmd, consumed := m.contextMenuClick(tea.Mouse{X: m.contextMenu.x + 1, Y: lastRow})
	require.True(t, consumed)
	require.NotNil(t, cmd)
	require.Len(t, m.workspaces, 2, "the final row must reopen the closed workspace")
}
