package notify

import (
	"github.com/yorukot/superfile/src/internal/common"
)

type Model struct {
	open          bool
	title         string
	content       string
	confirmAction ConfirmActionType
}

func New(open bool, title string, content string, confirmAction ConfirmActionType) Model {
	return Model{
		open:          open,
		title:         title,
		content:       content,
		confirmAction: confirmAction,
	}
}

func (m *Model) GetTitle() string {
	return m.title
}

func (m *Model) GetContent() string {
	return m.content
}

func (m *Model) IsOpen() bool {
	return m.open
}

func (m *Model) Open() {
	m.open = true
}

func (m *Model) Close() {
	m.open = false
}

func (m *Model) GetConfirmAction() ConfirmActionType {
	return m.confirmAction
}

// TODO: Remove code duplication with typineModalRender
func (m *Model) Render() string {
	return m.RenderAtWidth(common.ModalWidth)
}

// RenderAtWidth lets the browser share responsive dialog geometry with mouse handling.
func (m *Model) RenderAtWidth(width int) string {
	var inputKeysText string
	if m.confirmAction == NoAction {
		inputKeysText = common.ModalOkayInputText
	} else {
		inputKeysText = common.ModalConfirmInputText + common.ModalInputSpacingText + common.ModalCancelInputText
	}
	return common.ModalBorderStyle(common.ModalHeight, max(1, width)).
		Render(m.title + "\n\n" + m.content + "\n\n" + inputKeysText)
}
