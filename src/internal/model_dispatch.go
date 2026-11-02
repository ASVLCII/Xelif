package internal

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/ui/preview"
	zoxideui "github.com/yorukot/superfile/src/internal/ui/zoxide"
)

// Async results share the original dispatch order without receiving input keys.
// Explicit operation completions remain application-global, not workspace-scoped.
func (m *model) handleBackgroundUpdate(msg tea.Msg) tea.Cmd {
	switch event := msg.(type) {
	case zoxideui.UpdateMsg:
		slog.Debug("Got ModelUpdate message", "id", event.GetReqID())
		return event.Apply(&m.zoxideModal)
	case preview.UpdateMsg:
		slog.Debug("Got ModelUpdate message", "id", event.GetReqID())
		return m.fileModel.UpdatePreviewPanel(event)
	case refreshTickMsg:
		return refreshTickCmd()
	case editorFinishedMsg, gitStatusUpdateMsg, batchRenameOperationMsg, archiveOperationMsg,
		remoteOperationMsg, pluginCommandFinishedMsg, duplicateScanMsg, directoryCompareMsg,
		directorySyncMsg, metadataOperationMsg:
		m.applyOperationResult(event)
	case ModelUpdateMessage:
		slog.Debug("Got ModelUpdate message", "id", event.GetReqID())
		return event.ApplyToModel(m)
	default:
		slog.Debug("Message of type that is not explicitly handled")
	}
	return nil
}

// These results update state but do not return a command. Keep the command-
// producing branches in Update, where their original batch order is explicit.
func (m *model) applyOperationResult(msg tea.Msg) {
	switch msg := msg.(type) {
	case editorFinishedMsg:
		m.showUserError("Editor failed", msg.err)
	case gitStatusUpdateMsg:
		m.applyGitStatusUpdate(msg)
	case batchRenameOperationMsg:
		m.applyBatchRenameOperation(msg)
	case archiveOperationMsg:
		m.applyArchiveOperation(msg)
	case remoteOperationMsg:
		m.applyRemoteOperation(msg)
	case pluginCommandFinishedMsg:
		m.applyPluginCommandResult(msg)
	case duplicateScanMsg:
		m.applyDuplicateScan(msg)
	case directoryCompareMsg:
		m.applyDirectoryCompare(msg)
	case directorySyncMsg:
		m.applyDirectorySync(msg)
	case metadataOperationMsg:
		m.applyMetadataOperation(msg)
	}
}

func (m *model) handleFolderWatchMessage(msg tea.Msg) (tea.Cmd, tea.Cmd) {
	switch msg := msg.(type) {
	case folderWatchEventMsg:
		if !msg.closed {
			m.applyFolderWatchEvent(msg.event)
			return m.waitFolderWatchEventCmd(), m.refreshGitStatusCmd(true)
		}
	case folderWatchErrorMsg:
		if !msg.closed {
			m.showUserError("Folder watching failed", msg.err)
			return m.waitFolderWatchErrorCmd(), nil
		}
	case folderWatchSyncMsg:
		if msg.generation != 0 && msg.generation != m.folderWatchGeneration.Load() {
			return nil, nil
		}
		m.showUserError("Could not watch folders", msg.err)
	}
	return nil, nil
}
