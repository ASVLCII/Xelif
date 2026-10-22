package filepanel

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
)

func TestCompactEntryGeometryExcludesFooterAndBlankRows(t *testing.T) {
	old := common.Config
	t.Cleanup(func() { common.SetConfig(old) })
	for _, columns := range []int{0, 2} {
		common.Config.FilePanelExtraColumns = columns
		common.Config.FilePanelNamePercent = 50
		for _, width := range []int{18, 24, 40, 80, 120} {
			for _, height := range []int{6, 7, 12, 24, 40} {
				p := testModelWithElemCount(9, 0, height, 10)
				p.SetWidth(width)
				p.SetHeight(height)
				visible := min(p.PanelElementHeight(), p.ElemCount()-p.GetRenderIndex())
				for row := range visible {
					index, ok := p.EntryIndexAtY(p.EntryStartY() + row)
					require.True(t, ok)
					require.Equal(t, p.GetRenderIndex()+row, index)
				}
				_, hit := p.EntryIndexAtY(height - 1)
				require.False(t, hit)
				_, hit = p.EntryIndexAtY(p.EntryStartY() + visible)
				require.False(t, hit)
			}
		}
	}
}
