package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/require"
)

func TestOperationIdentityAllowsWatchedFolderAndSameFileEdit(t *testing.T) {
	root := t.TempDir()
	watcher, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, watcher.Close()) })
	require.NoError(t, watcher.Add(root))
	directory, err := captureOperationIdentity(root, true)
	require.NoError(t, err)
	require.NoError(t, directory.check(root))
	path := filepath.Join(root, "file.txt")
	require.NoError(t, os.WriteFile(path, []byte("before"), 0o600))
	identity, err := captureOperationIdentity(path, false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("after editing the same file"), 0o600))
	require.NoError(t, identity.check(path))
	require.Error(t, (operationIdentity{}).check(path))
}
