package internal

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/ui/processbar"
)

func TestPasteRejectsDotDotPrefixedDescendant(t *testing.T) {
	source := t.TempDir()
	child := filepath.Join(source, "..nested")
	require.NoError(t, os.Mkdir(child, 0o755))
	for _, cut := range []bool{false, true} {
		require.Error(t, validatePasteOperation(child, []string{source}, cut),
			"a child whose name starts with two dots is still inside its source")
	}
	require.False(t, isAncestor(child, source), "a parent is outside its child")
}

func TestCopyReadOnlyDirectoryTree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix directory permissions")
	}
	for _, operation := range []string{"paste", "move-fallback"} {
		t.Run(operation, func(t *testing.T) {
			root := t.TempDir()
			source, destination := filepath.Join(root, "source"), filepath.Join(root, "destination")
			nested := filepath.Join(source, "nested")
			require.NoError(t, os.MkdirAll(nested, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(nested, "contents.txt"), []byte("keep me"), 0o644))
			require.NoError(t, os.Chmod(nested, 0o555))
			require.NoError(t, os.Chmod(source, 0o555))
			t.Cleanup(func() {
				for _, path := range []string{source, nested, destination, filepath.Join(destination, "nested")} {
					_ = os.Chmod(path, 0o755)
				}
			})
			var err error
			if operation == "paste" {
				bar := processbar.New()
				p := processbar.Process{}
				err = pasteDir(source, destination, &p, false, &bar)
			} else {
				err = copyElementNoReplace(source, destination)
			}
			require.NoError(t, err)
			content, err := os.ReadFile(filepath.Join(destination, "nested", "contents.txt"))
			require.NoError(t, err)
			require.Equal(t, "keep me", string(content))
			for _, path := range []string{destination, filepath.Join(destination, "nested")} {
				info, statErr := os.Stat(path)
				require.NoError(t, statErr)
				require.Equal(t, os.FileMode(0o555), info.Mode().Perm())
			}
		})
	}
}

func TestPasteDirectoryRefusesAnExistingWalkDestination(t *testing.T) {
	root := t.TempDir()
	source, destination := filepath.Join(root, "source"), filepath.Join(root, "destination")
	require.NoError(t, os.Mkdir(source, 0o755))
	require.NoError(t, os.Mkdir(destination, 0o755))
	info, err := os.Stat(source)
	require.NoError(t, err)
	bar := processbar.New()
	p := processbar.Process{}
	err = actualPasteOperation(info, source, destination, &p, &bar)
	require.ErrorIs(t, err, os.ErrExist,
		"a directory appearing during the walk must abort instead of redirecting only its creation")
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Len(t, entries, 2, "do not create an alternate directory that descendants will not use")
}
