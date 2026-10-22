package internal

import tea "charm.land/bubbletea/v2"

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
		m.showUserError("Could not watch folders", msg.err)
	}
	return nil, nil
}
