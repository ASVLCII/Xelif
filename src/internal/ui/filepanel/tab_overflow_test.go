package filepanel

import (
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestTabOverflowKeepsActiveTabAndControlsReachable(t *testing.T) {
	for _, width := range []int{MinWidth, 24, 40, 80} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			panel := Model{width: width}
			for range 20 {
				panel.tabs = append(panel.tabs, panelTab{location: "報告-long-folder-name"})
			}
			for index := range panel.tabs {
				panel.activeTab = index
				view := ansi.Strip(panel.renderTabBar())
				require.LessOrEqual(t, ansi.StringWidth(view), panel.GetContentWidth())
				tab, ok := panel.TabAtContentX(0)
				require.True(t, ok)
				require.Equal(t, index, tab)
				createIndex := strings.Index(view, "[+]")
				require.NotEqual(t, -1, createIndex)
				createX := ansi.StringWidth(view[:createIndex])
				create, closeTab := panel.TabControlAtContentX(createX)
				require.True(t, create)
				require.False(t, closeTab)
				closeIndex := strings.Index(view, "[-]")
				require.NotEqual(t, -1, closeIndex)
				closeX := ansi.StringWidth(view[:closeIndex])
				create, closeTab = panel.TabControlAtContentX(closeX)
				require.False(t, create)
				require.True(t, closeTab)
			}
		})
	}
}
