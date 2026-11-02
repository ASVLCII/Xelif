package internal

import "log/slog"

func (m *model) cancelWorkspaceToolViews() {
	m.cancelDuplicateScan()
	m.cancelDirectoryPreview()
	m.cancelGitDiff()
	m.pendingBatchRename = nil
	m.pendingGitDiscard = nil
	m.textViewer.Close()
	if err := m.archiveViewer.Close(); err != nil {
		slog.Warn("Could not close archive view while switching workspaces", "error", err)
	}
	m.closeRemoteBrowser()
}
