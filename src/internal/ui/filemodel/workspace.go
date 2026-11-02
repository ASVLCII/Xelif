package filemodel

import (
	"github.com/yorukot/superfile/src/internal/session"
	"github.com/yorukot/superfile/src/internal/ui/filepanel"
)

// CancelEphemeral invalidates preview replies without resetting the request counter.
// Keep this Model instance across workspaces so old request IDs are never reused.
func (m *Model) CancelEphemeral() {
	if m.previewCancel != nil {
		m.previewCancel()
		m.previewCancel = nil
	}
	m.ioReqCnt++
	m.activePreviewReqID = -1
	m.FilePreview.SetLocation("")
}

func (m *Model) RestoreWorkspace(states []session.PanelState, focused int, dot bool, fallback string) {
	m.CancelEphemeral()
	panels := make([]filepanel.Model, max(1, len(states)))
	focused = min(max(focused, 0), len(panels)-1)
	for i := range panels {
		state := session.PanelState{}
		if i < len(states) {
			state = states[i]
		}
		panels[i] = filepanel.RestoreWorkspace(state, fallback, i == focused)
	}
	m.FilePanels, m.FocusedPanelIndex, m.DisplayDotFiles = panels, focused, dot
	m.Renaming = false
}
