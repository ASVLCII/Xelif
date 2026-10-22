package commandpalette

const (
	MinWidth  = 36
	MinHeight = 8

	inputPadding = 4
)

func (m *Model) SetWidth(width int) {
	m.width = max(width, MinWidth)
	m.textInput.SetWidth(m.width - inputPadding)
}

func (m *Model) GetWidth() int {
	return m.width
}

func (m *Model) SetMaxHeight(height int) {
	m.maxHeight = max(height, MinHeight)
	m.updateRenderIndex()
}

func (m *Model) GetMaxHeight() int {
	return m.maxHeight
}
