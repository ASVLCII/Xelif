package internal

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/batchrename"
	"github.com/yorukot/superfile/src/internal/common"
)

type batchRenameOperationMsg struct {
	plan *batchrename.Plan
	err  error
}

func (m *model) executeBatchRenameAction(action common.BatchRenameAction) (string, tea.Cmd, error) {
	switch action.Operation {
	case common.BatchRenameReplace, common.BatchRenamePrefix, common.BatchRenameSuffix:
		rule := batchrename.Rule{}
		switch action.Operation {
		case common.BatchRenameReplace:
			rule.Find, rule.Replace = action.Find, action.Replacement
		case common.BatchRenamePrefix:
			rule.Prefix = action.Replacement
		case common.BatchRenameSuffix:
			rule.Suffix = action.Replacement
		case common.BatchRenameApply, common.BatchRenameCancel:
			return "", nil, errors.New("batch rename apply and cancel do not describe preview rules")
		}
		plan, err := batchrename.NewPlan(m.gitOperationPaths(), rule)
		if err != nil {
			return "", nil, err
		}
		m.pendingBatchRename = &plan
		m.textViewer.Open("Batch rename preview", renderBatchRenamePlan(plan))
		m.setTextViewerSize()
		return fmt.Sprintf("Previewed %d rename(s); run batch-rename apply to confirm", len(plan.Entries)), nil, nil
	case common.BatchRenameApply:
		if m.pendingBatchRename == nil {
			return "", nil, errors.New("no batch rename preview is pending")
		}
		plan := m.pendingBatchRename
		return "Applying batch rename", func() tea.Msg {
			err := batchrename.Apply(context.Background(), *plan, moveElementNoReplace)
			return batchRenameOperationMsg{plan: plan, err: err}
		}, nil
	case common.BatchRenameCancel:
		m.pendingBatchRename = nil
		return "Batch rename cancelled", nil, nil
	default:
		return "", nil, errors.New("unknown batch rename operation: " + string(action.Operation))
	}
}

func (m *model) applyBatchRenameOperation(msg batchRenameOperationMsg) {
	if msg.plan != m.pendingBatchRename {
		return
	}
	if msg.err != nil {
		m.showUserError("Batch rename failed", msg.err)
		return
	}
	moves := make([]pathMove, len(msg.plan.Entries))
	for index, entry := range msg.plan.Entries {
		moves[index] = pathMove{from: entry.Source, to: entry.Destination}
	}
	m.recordUndo("batch rename", moves)
	m.updateCatalogMoves(moves)
	m.pendingBatchRename = nil
	m.fileModel.UpdateFilePanelsIfNeeded(true)
	m.textViewer.Open("Batch rename complete", fmt.Sprintf("Renamed %d item(s). Undo is available.\n", len(moves)))
	m.setTextViewerSize()
}

func renderBatchRenamePlan(plan batchrename.Plan) string {
	var builder strings.Builder
	builder.WriteString("No files have been changed. Review this plan, then run `batch-rename apply`.\n\n")
	for _, entry := range plan.Entries {
		builder.WriteString(filepath.Base(entry.Source))
		builder.WriteString("  →  ")
		builder.WriteString(filepath.Base(entry.Destination))
		builder.WriteByte('\n')
	}
	return builder.String()
}
