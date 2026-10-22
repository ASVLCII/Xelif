package archiveviewer

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testArchive(t *testing.T) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "files.zip")
	file, err := os.Create(archivePath)
	require.NoError(t, err)
	writer := zip.NewWriter(file)
	for name, content := range map[string]string{"folder/readme.txt": "hello", "root.txt": "root"} {
		member, createErr := writer.Create(name)
		require.NoError(t, createErr)
		_, writeErr := member.Write([]byte(content))
		require.NoError(t, writeErr)
	}
	require.NoError(t, writer.Close())
	require.NoError(t, file.Close())
	return archivePath
}

func TestArchiveViewerNavigatesAndReturnsActions(t *testing.T) {
	model := New()
	require.NoError(t, model.Open(testArchive(t)))
	t.Cleanup(func() { _ = model.Close() })
	model.SetDimensions(50, 10)
	assert.True(t, model.IsOpen())
	assert.Contains(t, ansi.Strip(model.Render()), "folder/")

	assert.Equal(t, NoAction, model.Enter().Kind)
	assert.Contains(t, ansi.Strip(model.Render()), "readme.txt")
	action := model.Enter()
	assert.Equal(t, PreviewAction, action.Kind)
	assert.Equal(t, "folder/readme.txt", action.Member)
	assert.Equal(t, NoAction, model.Back().Kind)
	assert.Equal(t, CloseAction, model.Back().Kind)
}

func TestArchiveViewerDoubleClickAndActionButtons(t *testing.T) {
	model := New()
	require.NoError(t, model.Open(testArchive(t)))
	t.Cleanup(func() { _ = model.Close() })
	model.SetDimensions(50, 10)
	now := time.Now()
	assert.Equal(t, NoAction, model.HandleClick(4, entryStartRow, now).Kind)
	assert.Equal(t, NoAction, model.HandleClick(4, entryStartRow, now.Add(time.Millisecond)).Kind)
	assert.Contains(t, ansi.Strip(model.Render()), "readme.txt")
	assert.Equal(t, ExtractAction, model.HandleClick(12, actionRow, now).Kind)
	assert.Equal(t, CloseAction, model.HandleClick(29, actionRow, now).Kind)
}
