package internal

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/notify"
	"github.com/yorukot/superfile/src/internal/ui/spferror"
	"github.com/yorukot/superfile/src/pkg/utils"
)

// renderedLabelHit derives hit targets from the actual terminal-cell output,
// including centered content, configurable key labels, and wrapped messages.
func renderedLabelHit(rendered, label string, x, y int) bool {
	lines := strings.Split(ansi.Strip(rendered), "\n")
	if y < 0 || y >= len(lines) || label == "" {
		return false
	}
	plainLabel := ansi.Strip(label)
	for row := len(lines) - 1; row >= 0; row-- {
		index := strings.Index(lines[row], plainLabel)
		if index < 0 {
			continue
		}
		// Actions are rendered after message content. A filename/message
		// containing the same button text must never become another action.
		if row != y {
			return false
		}
		start := ansi.StringWidth(lines[row][:index])
		return x >= start && x < start+ansi.StringWidth(plainLabel)
	}
	return false
}

func (m *model) handleModalMouseClick(mouse tea.Mouse) (tea.Cmd, bool) {
	if mouse.Button != tea.MouseLeft {
		return nil, false
	}
	x := mouse.X - (m.fullWidth/common.CenterDivisor - common.ModalWidth/common.CenterDivisor)
	y := mouse.Y - (m.fullHeight/common.CenterDivisor - common.ModalHeight/common.CenterDivisor)
	switch {
	case m.spfError.IsOpen():
		for _, item := range []struct{ label, key string }{
			{" (" + spferror.KeySkip()[0] + ") Skip ", spferror.KeySkip()[0]},
			{" (" + spferror.KeyAbort()[0] + ") Abort ", spferror.KeyAbort()[0]},
		} {
			if renderedLabelHit(m.spfError.Render(), item.label, x, y) {
				return m.handleKeyInput(utils.TeaRuneKeyMsg(item.key)), true
			}
		}
		return nil, true
	case m.helpMenu.IsOpen():
		if !insideCentered(mouse, m.fullWidth, m.fullHeight, m.helpMenu.GetWidth(), m.helpMenu.GetHeight()) {
			m.helpMenu.Close()
		}
		return nil, true
	case m.promptModal.IsOpen(), m.zoxideModal.IsOpen(), m.globalSearchModal.IsOpen(),
		m.commandPalette.IsOpen(), m.textViewer.IsOpen(), m.archiveViewer.IsOpen(), m.remoteViewer.IsOpen():
		// These overlays render ahead of sort/create/confirmation dialogs.
		// Let their handlers receive clicks; never activate a hidden dialog.
		return nil, false
	case m.sortModal.IsOpen():
		return m.handleSortModalMouseClick(mouse)
	case m.firstUse:
		m.firstUse = false
		return nil, true
	case m.typingModal.open:
		if renderedLabelHit(m.typineModalRender(), " Cancel ", x, y) {
			m.cancelTypingModal()
		} else if renderedLabelHit(m.typineModalRender(), " Create ", x, y) {
			return m.getCreateCmd(), true
		}
		return nil, true
	case m.notifyModel.IsOpen():
		return m.handleNotifyModalMouseClick(x, y)
	default:
		return nil, false
	}
}

func (m *model) handleSortModalMouseClick(mouse tea.Mouse) (tea.Cmd, bool) {
	left, top := m.sortMenuPosition()
	x, y := mouse.X-left, mouse.Y-top
	rendered := m.sortModal.Render()
	if x < 0 || y < 0 || x >= lipgloss.Width(rendered) || y >= lipgloss.Height(rendered) {
		m.sortModal.Close()
		return nil, true
	}
	if x == 0 || x == lipgloss.Width(rendered)-1 {
		return nil, true
	}
	if index, ok := m.sortModal.ItemAtRow(y - 1); ok {
		m.sortModal.Cursor = index
		m.confirmSortOptions()
	}
	return nil, true
}

func (m *model) handleNotifyModalMouseClick(x, y int) (tea.Cmd, bool) {
	rendered := m.notifyModel.Render()
	key := ""
	if renderedLabelHit(rendered, common.ModalCancelInputText, x, y) {
		key = common.Hotkeys.Quit[0]
	} else if renderedLabelHit(rendered, common.ModalConfirmInputText, x, y) ||
		(m.notifyModel.GetConfirmAction() == notify.NoAction &&
			renderedLabelHit(rendered, common.ModalOkayInputText, x, y)) {
		key = common.Hotkeys.ConfirmTyping[0]
	}
	if key != "" {
		return m.handleKeyInput(utils.TeaRuneKeyMsg(key)), true
	}
	return nil, true
}

func insideCentered(mouse tea.Mouse, screenWidth, screenHeight, width, height int) bool {
	x := screenWidth/common.CenterDivisor - width/common.CenterDivisor
	y := screenHeight/common.CenterDivisor - height/common.CenterDivisor
	return mouse.X >= x && mouse.X < x+width && mouse.Y >= y && mouse.Y < y+height
}
