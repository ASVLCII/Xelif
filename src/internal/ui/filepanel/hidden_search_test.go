package filepanel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchWhenEveryEntryIsHidden(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".hidden"), nil, 0o600))
	panel := newTabTestPanel(t, dir)
	panel.SearchBar.SetValue("hidden")
	assert.Empty(t, panel.getDirectoryElementsBySearch(false))
	assert.Len(t, panel.getDirectoryElementsBySearch(true), 1)
}
