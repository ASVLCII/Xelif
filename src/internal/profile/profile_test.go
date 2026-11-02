package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func testPaths(t *testing.T) Paths {
	t.Helper()
	root := t.TempDir()
	return Paths{
		ConfigDir:   filepath.Join(root, "config"),
		DataDir:     filepath.Join(root, "data"),
		StateDir:    filepath.Join(root, "state"),
		CacheDir:    filepath.Join(root, "cache"),
		ConfigFile:  filepath.Join(root, "config", "config.toml"),
		HotkeysFile: filepath.Join(root, "config", "hotkeys.toml"),
	}
}

func defaultPreferences(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "superfile_config", "config.toml"))
	require.NoError(t, err)
	return data
}

func TestInspectAndExplicitAppearance(t *testing.T) {
	p := testPaths(t)
	status, err := Inspect(p)
	require.NoError(t, err)
	require.False(t, status.Initialized)
	require.NoError(t, BeginSetup(p))
	require.NoError(t, atomicWrite(p.ConfigFile, defaultPreferences(t)))
	status, err = Inspect(p)
	require.NoError(t, err)
	require.True(t, status.PendingSetup)
	require.False(t, status.Initialized)
	require.NoError(t, SaveAppearance(p, "rich"))
	data, err := os.ReadFile(p.ConfigFile)
	require.NoError(t, err)
	require.Contains(t, string(data), "nerdfont = true")
	require.Contains(t, string(data), "plain_mode = false")
	status, err = Inspect(p)
	require.NoError(t, err)
	require.True(t, status.Initialized)
	require.False(t, status.PendingSetup)
	require.NoError(t, SaveAppearance(p, "portable"))
	data, err = os.ReadFile(p.ConfigFile)
	require.NoError(t, err)
	require.Contains(t, string(data), "nerdfont = false")
	require.Contains(t, string(data), "plain_mode = true")
}

func TestAppearancePreservesTablesCommentsAndCorruptBytes(t *testing.T) {
	data := append(defaultPreferences(t), []byte("\n# kept comment\n[open_with]\ncustom = 'my-editor'\n")...)
	updated, err := setAppearance(data, true)
	require.NoError(t, err)
	require.Contains(t, string(updated), "# kept comment\n[open_with]\ncustom = 'my-editor'")
	p := testPaths(t)
	bad := []byte("not valid = [")
	require.NoError(t, atomicWrite(p.ConfigFile, bad))
	require.Error(t, SaveAppearance(p, "rich"))
	after, err := os.ReadFile(p.ConfigFile)
	require.NoError(t, err)
	require.Equal(t, bad, after)
}

func TestLegacyImportBacksUpAndNeverCopiesPrivateState(t *testing.T) {
	source, destination := testPaths(t), testPaths(t)
	original := defaultPreferences(t)
	require.NoError(t, atomicWrite(source.ConfigFile, original))
	require.NoError(t, atomicWrite(destination.ConfigFile, []byte("# destination\n")))
	for _, name := range []string{"credentials.json", "operations.json", "undo.json", "secret.log"} {
		require.NoError(t, atomicWrite(filepath.Join(source.DataDir, name), []byte("private")))
	}
	require.NoError(
		t,
		atomicWrite(filepath.Join(source.ConfigDir, "command-plugins", "tool.toml"), []byte("command='danger'")),
	)
	preview, err := PreviewLegacyImport(source, destination)
	require.NoError(t, err)
	require.True(t, preview.Found)
	require.Equal(t, 1, preview.ReadyCount)
	before, err := os.ReadFile(source.ConfigFile)
	require.NoError(t, err)
	result, err := ImportLegacy(source, destination)
	require.NoError(t, err)
	require.True(t, result.SourcePreserved)
	require.NotEmpty(t, result.BackupDir)
	after, err := os.ReadFile(source.ConfigFile)
	require.NoError(t, err)
	require.Equal(t, before, after)
	imported, err := os.ReadFile(destination.ConfigFile)
	require.NoError(t, err)
	require.Equal(t, original, imported)
	backup, err := os.ReadFile(filepath.Join(result.BackupDir, "before", "0000"))
	require.NoError(t, err)
	require.Equal(t, "# destination\n", string(backup))
	for _, name := range []string{"credentials.json", "operations.json", "undo.json", "secret.log"} {
		_, err := os.Stat(filepath.Join(destination.DataDir, name))
		require.True(t, os.IsNotExist(err))
	}
}

func TestImportRejectsOverlapAndPreservesUnsupportedSource(t *testing.T) {
	source, destination := testPaths(t), testPaths(t)
	bad := []byte("future_option = true\n")
	require.NoError(t, atomicWrite(source.ConfigFile, bad))
	preview, err := PreviewLegacyImport(source, destination)
	require.NoError(t, err)
	require.Equal(t, 1, preview.RejectedCount)
	require.Zero(t, preview.ReadyCount)
	_, err = PreviewLegacyImport(source, source)
	require.Error(t, err)
	data, err := os.ReadFile(source.ConfigFile)
	require.NoError(t, err)
	require.Equal(t, bad, data)
	require.NotContains(t, string(data), "confirmed")
}
