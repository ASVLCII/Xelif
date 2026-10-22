package globalsearch

const (
	MinWidth  = 30
	MinHeight = 8

	inputPadding = 6
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
}

func (m *Model) GetMaxHeight() int {
	return m.maxHeight
}
