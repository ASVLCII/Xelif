package metadata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
)

func TestDirectorySizeRequiresExplicitCalculation(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0o600))

	meta := GetMetadata(dir, false, nil)
	size, err := meta.GetValue(keySize)
	require.NoError(t, err)
	require.Equal(t, "Not calculated", size)

	meta = GetMetadata(dir, true, nil)
	size, err = meta.GetValue(keySize)
	require.NoError(t, err)
	require.Equal(t, common.FormatFileSize(5), size)
}
