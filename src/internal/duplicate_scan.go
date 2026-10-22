package internal

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/duplicates"
)

const (
	duplicateMaxFiles            = 100_000
	duplicateMaxBytesHashed      = int64(20 * 1024 * 1024 * 1024)
	duplicateWarningLimit        = 20
	duplicateDigestPreviewLength = 12
)

type duplicateScanMsg struct {
	reqID  int
	root   string
	result duplicates.Result
	err    error
}

func (m *model) startDuplicateScan(root string) tea.Cmd {
	if m.duplicateScanCancel != nil {
		m.duplicateScanCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.duplicateScanCancel = cancel
	m.duplicateScanReqID++
	reqID := m.duplicateScanReqID
	m.textViewer.Open(
		"Duplicate scan",
		"Scanning read-only in the background…\nClose this view to keep working; use the palette to cancel.",
	)
	m.setTextViewerSize()
	return func() tea.Msg {
		result, err := duplicates.Scan(ctx, []string{root}, duplicates.Options{
			MaxFiles: duplicateMaxFiles, MaxBytesHashed: duplicateMaxBytesHashed,
		})
		return duplicateScanMsg{reqID: reqID, root: root, result: result, err: err}
	}
}

func (m *model) cancelDuplicateScan() {
	if m.duplicateScanCancel != nil {
		m.duplicateScanCancel()
		m.duplicateScanCancel = nil
	}
}

func (m *model) applyDuplicateScan(msg duplicateScanMsg) {
	if msg.reqID != m.duplicateScanReqID {
		return
	}
	m.duplicateScanCancel = nil
	if msg.err != nil {
		m.showUserError("Duplicate scan failed", msg.err)
		return
	}
	m.textViewer.Open("Duplicate files", renderDuplicateResult(msg.root, msg.result))
	m.setTextViewerSize()
}

func renderDuplicateResult(root string, result duplicates.Result) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Root: %s\nScanned: %d files; hashed: %d (%s)\nDuplicate groups: %d\n",
		root, result.FilesScanned, result.FilesHashed, common.FormatFileSize(result.BytesHashed), len(result.Groups))
	if result.Truncated {
		builder.WriteString("Result is partial because a safety limit was reached.\n")
	}
	for groupIndex, group := range result.Groups {
		digest := group.Digest
		if len(digest) > duplicateDigestPreviewLength {
			digest = digest[:duplicateDigestPreviewLength]
		}
		fmt.Fprintf(&builder, "\nGroup %d — %s each — SHA-256 %s…\n",
			groupIndex+1, common.FormatFileSize(group.Size), digest)
		for _, file := range group.Files {
			builder.WriteString("  " + file.Path + "\n")
		}
	}
	if len(result.Groups) == 0 {
		builder.WriteString("\nNo byte-for-byte duplicates were found within the scan limits.\n")
	}
	if len(result.Warnings) > 0 {
		fmt.Fprintf(&builder, "\nWarnings (%d):\n", len(result.Warnings))
		for _, warning := range result.Warnings[:min(len(result.Warnings), duplicateWarningLimit)] {
			builder.WriteString("  " + warning.Error() + "\n")
		}
	}
	return builder.String()
}
