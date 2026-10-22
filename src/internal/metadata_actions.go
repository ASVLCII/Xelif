package internal

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	variable "github.com/yorukot/superfile/src/config"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/metadataedit"
)

type metadataOperationMsg struct {
	operation common.MetadataOperation
	result    metadataedit.PreviewResult
	journal   string
	err       error
}

func (m *model) executeMetadataAction(action common.MetadataAction) (string, tea.Cmd, error) {
	switch action.Operation {
	case common.MetadataModified, common.MetadataAccessed, common.MetadataPermissions:
		paths := m.gitOperationPaths()
		if len(paths) != 1 {
			return "", nil, errors.New("metadata editing requires exactly one focused or selected item")
		}
		changes, err := parseMetadataChanges(action)
		if err != nil {
			return "", nil, err
		}
		patch, err := metadataedit.BuildPatch(paths[0], changes)
		if err != nil {
			return "", nil, err
		}
		preview, err := metadataedit.Preview(context.Background(), patch)
		if err != nil {
			return "", nil, err
		}
		m.pendingMetadataPatch = &patch
		m.textViewer.Open("Metadata preview", renderMetadataPreview(preview))
		m.setTextViewerSize()
		return "Metadata preview ready; use metadata apply to confirm", nil, nil
	case common.MetadataApply:
		if m.pendingMetadataPatch == nil {
			return "", nil, errors.New("no metadata preview is pending")
		}
		patch := *m.pendingMetadataPatch
		journal := filepath.Join(variable.MetadataJournalDir,
			fmt.Sprintf("metadata-%d.json", time.Now().UTC().UnixNano()))
		return "Applying metadata", func() tea.Msg {
			result, err := metadataedit.Apply(context.Background(), patch, journal)
			return metadataOperationMsg{
				operation: common.MetadataApply, result: result, journal: journal, err: err,
			}
		}, nil
	case common.MetadataRestore:
		if m.lastMetadataJournal == "" {
			return "", nil, errors.New("no metadata journal is available to restore")
		}
		journal := m.lastMetadataJournal
		return "Restoring metadata", func() tea.Msg {
			result, err := metadataedit.Restore(context.Background(), journal)
			return metadataOperationMsg{
				operation: common.MetadataRestore, result: result, journal: journal, err: err,
			}
		}, nil
	default:
		return "", nil, errors.New("unknown metadata operation: " + string(action.Operation))
	}
}

func parseMetadataChanges(action common.MetadataAction) (metadataedit.Changes, error) {
	var changes metadataedit.Changes
	switch action.Operation {
	case common.MetadataModified, common.MetadataAccessed:
		value, err := time.Parse(time.RFC3339, action.Value)
		if err != nil {
			return changes, fmt.Errorf("time must use RFC3339, for example 2026-07-31T12:00:00Z: %w", err)
		}
		if action.Operation == common.MetadataModified {
			changes.ModifiedTime = &value
		} else {
			changes.AccessTime = &value
		}
	case common.MetadataPermissions:
		value, err := strconv.ParseUint(action.Value, 8, 9)
		if err != nil {
			return changes, fmt.Errorf("permissions must be octal 0000-0777: %w", err)
		}
		permissions := fs.FileMode(value)
		changes.Permissions = &permissions
	case common.MetadataApply, common.MetadataRestore:
		return changes, errors.New("metadata apply and restore do not describe preview changes")
	default:
		return changes, errors.New("unknown metadata operation: " + string(action.Operation))
	}
	return changes, nil
}

func (m *model) applyMetadataOperation(msg metadataOperationMsg) {
	if msg.err != nil {
		m.showUserError("Metadata operation failed", msg.err)
		return
	}
	switch msg.operation {
	case common.MetadataApply:
		m.pendingMetadataPatch = nil
		m.lastMetadataJournal = msg.journal
	case common.MetadataRestore:
		m.lastMetadataJournal = ""
	case common.MetadataModified, common.MetadataAccessed, common.MetadataPermissions:
		m.showUserError(
			"Metadata operation failed",
			errors.New("preview actions cannot be applied as completed operations"),
		)
		return
	default:
		m.showUserError("Metadata operation failed", errors.New("unknown completed metadata operation"))
		return
	}
	m.fileModel.UpdateFilePanelsIfNeeded(true)
	title := "Metadata updated"
	if msg.operation == common.MetadataRestore {
		title = "Metadata restored"
	}
	m.textViewer.Open(title, renderMetadataPreview(msg.result)+"\nJournal: "+msg.journal)
	m.setTextViewerSize()
}

func renderMetadataPreview(preview metadataedit.PreviewResult) string {
	return fmt.Sprintf(
		"Path: %s\n\nBefore\n  Modified: %s\n  Accessed: %s\n  Permissions: %04o\n\nAfter\n  Modified: %s\n  Accessed: %s\n  Permissions: %04o\n",
		preview.Before.Path,
		preview.Before.ModifiedTime.Format(time.RFC3339),
		preview.Before.AccessTime.Format(time.RFC3339),
		preview.Before.Permissions.Perm(),
		preview.After.ModifiedTime.Format(time.RFC3339),
		preview.After.AccessTime.Format(time.RFC3339),
		preview.After.Permissions.Perm(),
	)
}
