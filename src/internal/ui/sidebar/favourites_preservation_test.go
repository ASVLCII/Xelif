package sidebar

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFavouritesLoadPreservesOfflineEntriesAndOriginalBytes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "pinned.json")
	offline := filepath.Join(root, "offline-drive", "project")
	mgr := PinnedManager{filePath: path}
	wanted := []directory{{Location: offline, Name: "My project"}}
	require.NoError(t, mgr.Save(wanted))
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	for range 3 {
		assert.Equal(t, wanted, mgr.Load())
	}
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, before, after, "rendering must not rewrite or prune favourites")
	require.NoError(t, mgr.Rename(offline, "Offline project"))
	require.NoError(t, mgr.Toggle(offline), "an offline favourite can still be explicitly removed")
	assert.Empty(t, mgr.Load())
}

func TestFavouritesCorruptStorageFailsClosed(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "pinned.json")
	before := []byte("[{\"location\":\"unfinished")
	require.NoError(t, os.WriteFile(path, before, 0o600))
	mgr := PinnedManager{filePath: path}
	assert.Empty(t, mgr.Load())
	require.Error(t, mgr.Toggle(root))
	require.Error(t, mgr.Rename(root, "renamed"))
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

func TestFavouritesFailedSaveLeavesExistingTargetAndCleansTemporary(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "pinned.json")
	require.NoError(t, os.Mkdir(target, 0o700))
	marker := filepath.Join(target, "keep.txt")
	require.NoError(t, os.WriteFile(marker, []byte("keep"), 0o600))
	mgr := PinnedManager{filePath: target}
	require.Error(t, mgr.Save([]directory{}))
	content, err := os.ReadFile(marker)
	require.NoError(t, err)
	assert.Equal(t, "keep", string(content))
	matches, err := filepath.Glob(filepath.Join(root, ".favourites-*"))
	require.NoError(t, err)
	assert.Empty(t, matches)
}

func TestFavouritesDoNotAddFiles(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file.txt")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	mgr := PinnedManager{filePath: filepath.Join(root, "pinned.json")}
	require.Error(t, mgr.Toggle(file))
	assert.Empty(t, mgr.Load())
}
