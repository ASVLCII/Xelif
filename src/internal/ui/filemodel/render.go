package filemodel

import "charm.land/lipgloss/v2"

func (m *Model) Render() string {
	visible := m.VisiblePanelIndices()
	f := make([]string, len(visible)+1)
	for i, index := range visible {
		filePanel := &m.FilePanels[index]
		f[i] = filePanel.Render(filePanel.IsFocused)
	}
	f[len(visible)] = m.GetFilePreviewRender()
	return lipgloss.JoinHorizontal(lipgloss.Top, f...)
}

func (m *Model) GetFilePreviewRender() string {
	if !m.PreviewVisible() {
		return ""
	}
	// Check if width and height have been synced yet
	if m.FilePreview.GetContentHeight() == m.Height &&
		m.FilePreview.GetContentWidth() == m.ExpectedPreviewWidth {
		if m.FilePreview.IsLoading() {
			return m.FilePreview.RenderText(FilePreviewLoadingText)
		}
		return m.FilePreview.GetContent()
	}

	// Placeholder resizing text till they get synced
	return m.FilePreview.RenderTextWithDimension(
		FilePreviewResizingText, m.Height, m.ExpectedPreviewWidth)
}
