package internal

import (
	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/common"
)

const (
	sidebarDrawerBreakpoint          = 80
	sidebarCompactBreakpoint         = 100
	sidebarCompactContentWidth       = 20
	workspaceRenameHorizontalPadding = 2
	browserEscapeKey                 = "esc"
	browserDownKey                   = "down"
	browserEnterKey                  = "enter"
)

// A hidden browser must not accept file actions while only the resize message
// is visible. Async operation completions and resize events still pass through.
func (m *model) inputBlockedByTerminalSize(msg tea.Msg) bool {
	if m.fullWidth <= 0 || m.fullHeight <= 0 ||
		(m.fullWidth >= common.MinimumWidth && m.fullHeight >= common.MinimumHeight) {
		return false
	}
	switch event := msg.(type) {
	case tea.KeyPressMsg:
		return !containsKey(common.Hotkeys.Quit, event.String())
	case tea.MouseMsg, tea.PasteMsg:
		return true
	default:
		return false
	}
}
