package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCopyFileNeverOverwritesExistingDestination(t *testing.T) {
	dir := t.TempDir()
	source, destination := filepath.Join(dir, "source"), filepath.Join(dir, "destination")
	require.NoError(t, os.WriteFile(source, []byte("new content"), 0600))
	require.NoError(t, os.WriteFile(destination, []byte("preserve this"), 0600))
	info, err := os.Stat(source)
	require.NoError(t, err)
	require.ErrorIs(t, copyFile(source, destination, info), os.ErrExist)
	data, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, "preserve this", string(data))
	data, err = os.ReadFile(source)
	require.NoError(t, err)
	require.Equal(t, "new content", string(data))
}

func TestCopyFilePublishesNewDestination(t *testing.T) {
	dir := t.TempDir()
	source, destination := filepath.Join(dir, "source"), filepath.Join(dir, "destination")
	require.NoError(t, os.WriteFile(source, []byte("new content"), 0600))
	info, err := os.Stat(source)
	require.NoError(t, err)
	require.NoError(t, copyFile(source, destination, info))
	data, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, "new content", string(data))
}
