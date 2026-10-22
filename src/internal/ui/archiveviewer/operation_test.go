package archiveviewer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/archivebrowser"
)

func TestArchiveOperationCannotStartClosed(t *testing.T) {
	model := New()
	_, err := model.StartOperation()
	require.ErrorIs(t, err, archivebrowser.ErrClosed)
}

func TestArchiveOperationReplacementAndCloseCancelContexts(t *testing.T) {
	model := New()
	archivePath := testArchive(t)
	require.NoError(t, model.Open(archivePath))
	first, err := model.StartOperation()
	require.NoError(t, err)
	assert.Equal(t, archivePath, first.ArchivePath)
	second, err := model.StartOperation()
	require.NoError(t, err)
	require.ErrorIs(t, first.Context.Err(), context.Canceled)
	assert.Greater(t, second.RequestID, first.RequestID)
	assert.False(t, model.CompleteOperation(first))
	require.NoError(t, model.Close())
	require.ErrorIs(t, second.Context.Err(), context.Canceled)
	assert.False(t, model.CompleteOperation(second))
}

func TestArchiveOperationCompletionConsumesRequest(t *testing.T) {
	model := New()
	require.NoError(t, model.Open(testArchive(t)))
	t.Cleanup(func() { _ = model.Close() })
	operation, err := model.StartOperation()
	require.NoError(t, err)
	assert.True(t, model.CompleteOperation(operation))
	require.ErrorIs(t, operation.Context.Err(), context.Canceled)
	assert.False(t, model.CompleteOperation(operation))
}
