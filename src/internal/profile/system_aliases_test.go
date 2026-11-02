package profile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDarwinCanonicalizationAcceptsOnlyKnownSystemAliases(t *testing.T) {
	for _, item := range []struct{ value, target, want string }{
		{"/var/folders/example/config", "private/var", "/private/var/folders/example/config"},
		{"/tmp/example/config", "/private/tmp", "/private/tmp/example/config"},
		{"/etc/example", "private/etc", "/private/etc/example"},
		{"/var/example", "/unrelated", "/var/example"},
		{"/var/example", "/private/other/../var", "/var/example"},
		{"/variable/example", "/private/var", "/variable/example"},
		{"/home/alias/example", "/private/var", "/home/alias/example"},
	} {
		t.Run(item.value+item.target, func(t *testing.T) {
			got := canonicalDarwinRoot(item.value, func(string) (string, error) { return item.target, nil })
			require.Equal(t, item.want, got)
		})
	}
	value := "/var/example"
	require.Equal(t, value, canonicalDarwinRoot(value, func(string) (string, error) { return "", os.ErrNotExist }))
}

func TestProfileSymlinkRefusalStillPreservesTarget(t *testing.T) {
	p := testPaths(t)
	target := t.TempDir()
	original := defaultPreferences(t)
	require.NoError(t, os.WriteFile(filepath.Join(target, "config.toml"), original, privateFileMode))
	if err := os.Symlink(target, p.ConfigDir); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	require.Error(t, SaveAppearance(p, "rich"))
	current, err := os.ReadFile(filepath.Join(target, "config.toml"))
	require.NoError(t, err)
	require.Equal(t, original, current)
}

func TestDarwinAliasCannotBypassImportOverlapProtection(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("requires native macOS system aliases")
	}
	p := testPaths(t)
	require.NoError(t, atomicWrite(p.ConfigFile, defaultPreferences(t)))
	physical, err := normalizePaths(p)
	require.NoError(t, err)
	_, err = PreviewLegacyImport(p, physical)
	require.Error(t, err, "logical and physical aliases name the same profile")
}
