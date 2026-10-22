package filepanel

import (
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/rendering"
)

// FooterAction identifies an interactive file-panel footer item.
type FooterAction uint8

const (
	FooterActionSort FooterAction = iota
	FooterActionMode
)

// FooterActionAtX returns the action rendered at a bottom-border X coordinate.
func (m *Model) FooterActionAtX(x int) (FooterAction, bool) {
	if !common.Config.ShowPanelFooterInfo {
		return 0, false
	}

	itemIndex, ok := rendering.BorderInfoItemAtX(m.width, m.footerInfoItems(m.SelectedCount()), x)
	if !ok {
		return 0, false
	}
	switch itemIndex {
	case 0:
		return FooterActionSort, true
	case 1:
		return FooterActionMode, true
	default:
		return 0, false
	}
}
