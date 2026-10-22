package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestRenameDoesNotTreatDistinctHardLinkAsCaseOnlyRename(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	destination := filepath.Join(dir, "destination.txt")
	utils.SetupFilesWithData(t, []byte("content"), source)
	if err := os.Link(source, destination); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}

	m := defaultTestModel(dir)
	setFilePanelSelectedItemByName(t, m.getFocusedFilePanel(), filepath.Base(source))
	m.panelItemRename()
	m.getFocusedFilePanel().Rename.SetValue(filepath.Base(destination))
	m.confirmRename()

	assert.FileExists(t, source)
	assert.FileExists(t, destination)
	assert.Nil(t, m.lastUndo)
	content, err := os.ReadFile(destination)
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))
}
