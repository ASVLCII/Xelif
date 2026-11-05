package internal

import (
	"strings"
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

func TestContextMenuShortcutColumnIsAlignedWithClearGap(t *testing.T) {
	labels := []string{
		contextMenuItems[contextOpen],
		"Copy",
		"Rename",
		contextMenuItems[contextTrash],
		"Open in new workspace",
	}
	shortcuts := []string{"enter", "ctrl+c", "f2", "delete", ""}
	for _, columns := range []int{60, 80, 120} {
		width, cueWidth := contextMenuColumnWidths(labels, shortcuts, columns-common.BorderPadding)
		endColumn := -1
		for i, label := range labels {
			line := contextMenuColumnLine("  ", label, shortcuts[i], width, cueWidth)
			require.Equal(t, width, ansi.StringWidth(line))
			require.Contains(t, line, label)
			if shortcuts[i] == "" {
				continue
			}
			index := strings.Index(line, shortcuts[i])
			gap := index - len("  "+label)
			require.GreaterOrEqual(t, gap, contextMenuColumnGap)
			end := ansi.StringWidth(line[:index]) + ansi.StringWidth(shortcuts[i])
			if endColumn < 0 {
				endColumn = end
			}
			require.Equal(t, endColumn, end)
		}
		require.LessOrEqual(t, width+common.BorderPadding, columns)
	}
}

func TestContextMenuColumnsHandleUnicodeAndOverlongBindings(t *testing.T) {
	labels := []string{"文件", "Cafe\u0301", contextMenuItems[contextOpen]}
	shortcuts := []string{"ctrl+x", "f2", "enter"}
	width, cueWidth := contextMenuColumnWidths(labels, shortcuts, 58)
	for i, label := range labels {
		line := contextMenuColumnLine("  ", label, shortcuts[i], width, cueWidth)
		require.Equal(t, width, ansi.StringWidth(line))
		require.Contains(t, line, label)
		require.True(t, strings.HasSuffix(strings.TrimRight(line, " "), shortcuts[i]))
	}
	trashLabel := contextMenuItems[contextTrash]
	width, cueWidth = contextMenuColumnWidths([]string{trashLabel}, []string{strings.Repeat("x", 100)}, 58)
	require.Zero(t, cueWidth)
	line := contextMenuColumnLine("  ", trashLabel, strings.Repeat("x", 100), width, cueWidth)
	require.Contains(t, line, trashLabel)
	require.NotContains(t, line, "xxx")
	require.LessOrEqual(t, ansi.StringWidth(line), 58)
}

func TestContextMenuShortcutRetainsPaddingInsideBorder(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	m.contextMenu = contextMenuState{open: true}
	shortcut := common.Hotkeys.CopyItems[0]
	for _, row := range strings.Split(ansi.Strip(m.contextMenuRender()), "\n") {
		if strings.Contains(row, shortcut) {
			require.True(t, strings.HasSuffix(row, "  "+common.Config.BorderRight), row)
			return
		}
	}
	t.Fatal("Copy shortcut missing from menu")
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
