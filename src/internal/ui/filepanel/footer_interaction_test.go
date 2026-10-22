package filepanel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/rendering"
)

func TestFooterActionAtXUsesRenderedInfoItemLayout(t *testing.T) {
	originalConfig := common.Config
	common.Config.Nerdfont = false
	common.Config.ShowPanelFooterInfo = true
	t.Cleanup(func() { common.SetConfig(originalConfig) })

	panel := Model{width: 60, selected: make(map[string]int), PanelMode: BrowserMode}
	sortLabel, sortIcon := panel.getSortInfo()
	modeLabel, _ := panel.getPanelModeInfo(0)
	items := []string{sortIcon + " " + sortLabel, modeLabel, panel.getCursorString()}
	sortX := infoItemX(t, panel.width, items, 0)
	modeX := infoItemX(t, panel.width, items, 1)

	action, ok := panel.FooterActionAtX(sortX)
	require.True(t, ok)
	assert.Equal(t, FooterActionSort, action)
	action, ok = panel.FooterActionAtX(modeX)
	require.True(t, ok)
	assert.Equal(t, FooterActionMode, action)
}

func infoItemX(t *testing.T, width int, items []string, target int) int {
	t.Helper()
	for x := range width {
		index, ok := rendering.BorderInfoItemAtX(width, items, x)
		if ok && index == target {
			return x
		}
	}
	require.FailNow(t, "info item was not rendered", "target index: %d", target)
	return 0
}
