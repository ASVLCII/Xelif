package internal

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDroppedPathsHandlesQuotedPathsAndFileURLs(t *testing.T) {
	first := filepath.Join(t.TempDir(), "first file.txt")
	second := filepath.Join(t.TempDir(), "second.txt")
	require.NoError(t, os.WriteFile(first, []byte("first"), 0o600))
	require.NoError(t, os.WriteFile(second, []byte("second"), 0o600))
	fileURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(second)}).String()
	decoded, ok := parseFileURL(fileURL)
	require.True(t, ok, fileURL)
	require.Equal(t, second, decoded, fileURL)

	paths := parseDroppedPaths(`"` + first + `" ` + fileURL)
	assert.ElementsMatch(t, []string{first, second}, paths)
}

func TestParseDroppedPathsRejectsCommandsAndMissingPaths(t *testing.T) {
	assert.Empty(t, parseDroppedPaths(`$(echo unsafe) C:\missing-file`))
	assert.Empty(t, parseDroppedPaths("relative.txt"))
}
