package internal

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/ui/archiveviewer"
)

// Keep fallback polling alive even when there is no keyboard or mouse input.
// Filesystem notifications still force immediate refreshes of watched panels.
type refreshTickMsg struct{}

func refreshTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return refreshTickMsg{} })
}

// Completed file operations may have changed disk contents even on partial
// failure. Do not let fallback throttling delay their visible results when
// notifications are unavailable. Progress/metadata messages remain cheap.
func (m *model) refreshPanelsAfterOperation(msg tea.Msg) {
	switch msg := msg.(type) {
	case CreateOperationMsg, DeleteOperationMsg, PasteOperationMsg, UndoOperationMsg,
		CompressOperationMsg, ExtractOperationMsg, gitOperationMsg, editorFinishedMsg, pluginCommandFinishedMsg:
		m.refreshAfterDiskChange()
	case directorySyncMsg:
		m.refreshAfterReconciledOperation(msg.err)
	case batchRenameOperationMsg:
		m.refreshAfterReconciledOperation(msg.err)
	case metadataOperationMsg:
		m.refreshAfterReconciledOperation(msg.err)
	case archiveOperationMsg:
		if msg.kind == archiveviewer.ExtractAction {
			m.refreshAfterReconciledOperation(msg.err)
		}
	case remoteOperationMsg:
		if msg.kind == remoteDownloadFile {
			m.refreshAfterReconciledOperation(msg.err)
		}
	}
}

func (m *model) refreshAfterDiskChange() {
	m.fileModel.UpdateFilePanelsIfNeeded(true)
	m.sidebarModel.UpdateDirectories()
}

// These newer handlers already reload panels on success, but can return early
// after a partial failure. Sidebar paths must be reconciled in either case.
func (m *model) refreshAfterReconciledOperation(err error) {
	if err != nil {
		m.fileModel.UpdateFilePanelsIfNeeded(true)
	}
	m.sidebarModel.UpdateDirectories()
}
