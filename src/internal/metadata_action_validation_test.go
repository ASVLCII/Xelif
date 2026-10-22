package internal

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
)

func TestMetadataChangeParserRejectsNonPreviewActions(t *testing.T) {
	for _, operation := range []common.MetadataOperation{
		common.MetadataApply, common.MetadataRestore, "unknown",
	} {
		t.Run(string(operation), func(t *testing.T) {
			_, err := parseMetadataChanges(common.MetadataAction{Operation: operation})
			require.Error(t, err)
		})
	}
}

func TestMetadataResultRejectsPreviewOperationWithoutChangingJournal(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	m.lastMetadataJournal = "preserve-existing-journal"
	m.applyMetadataOperation(metadataOperationMsg{operation: common.MetadataModified})
	require.Equal(t, "preserve-existing-journal", m.lastMetadataJournal)
	require.False(t, m.textViewer.IsOpen())
	require.True(t, m.notifyModel.IsOpen())
}
