package internal

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/metadataedit"
)

func TestMetadataCompletionCannotStealAnotherWorkspaceOrPreview(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	original := &metadataedit.Patch{}
	m.pendingMetadataPatch = original
	view := m.metadataSubmissionView()
	_ = m.newWorkspace(t.TempDir())
	require.Nil(t, m.pendingMetadataPatch)
	newer := &metadataedit.Patch{}
	m.pendingMetadataPatch = newer
	m.textViewer.Open("Keep this workspace", "Unrelated content")
	m.metadataOperationBusy = true
	m.applyMetadataOperation(
		metadataOperationMsg{
			operation: common.MetadataApply,
			journal:   "completed-journal",
			patch:     original,
			view:      view,
		},
	)
	require.Same(t, newer, m.pendingMetadataPatch)
	require.Equal(t, "completed-journal", m.lastMetadataJournal)
	require.False(t, m.metadataOperationBusy)
	require.Contains(t, m.textViewer.Render(), "Keep this workspace")
}

func TestMetadataOperationRejectsConcurrentSubmission(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	m.metadataOperationBusy = true
	for _, operation := range []common.MetadataOperation{common.MetadataApply, common.MetadataRestore} {
		_, cmd, err := m.executeMetadataAction(common.MetadataAction{Operation: operation})
		require.ErrorContains(t, err, "still running")
		require.Nil(t, cmd)
	}
}

func TestMetadataSupersededCompletionDoesNotReleaseNewOperation(t *testing.T) {
	m := dailyDriverTestModel(t.TempDir())
	old := m.beginMetadataOperation()
	_ = m.beginMetadataOperation()
	m.lastMetadataJournal = "new-journal"
	m.applyMetadataOperation(metadataOperationMsg{operation: common.MetadataApply, journal: "old-journal", view: old})
	require.True(t, m.metadataOperationBusy)
	require.Equal(t, "new-journal", m.lastMetadataJournal)
}
