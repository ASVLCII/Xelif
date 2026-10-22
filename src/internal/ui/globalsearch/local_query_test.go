package globalsearch

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchLocalQueryAppliesCombinedMetadataFilters(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	now := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)
	want := filepath.Join(root, "annual-report.TXT")
	require.NoError(t, os.WriteFile(want, make([]byte, 2049), 0o600))
	require.NoError(t, os.Chtimes(want, now.Add(-24*time.Hour), now.Add(-24*time.Hour)))
	old := filepath.Join(root, "annual-old.txt")
	require.NoError(t, os.WriteFile(old, make([]byte, 2049), 0o600))
	require.NoError(t, os.Chtimes(old, now.Add(-10*24*time.Hour), now.Add(-10*24*time.Hour)))
	require.NoError(t, os.WriteFile(filepath.Join(root, "annual-report.md"), make([]byte, 2049), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(root, "annual-report-folder"), 0o700))

	query, err := ParseQuery("annual report type:file ext:txt size:>2k size:<3k modified:<7d")
	require.NoError(t, err)
	response := SearchLocalQuery(context.Background(), root, query, LocalSearchOptions{
		ResultLimit:     10,
		VisitLimit:      100,
		MaxContentBytes: 1024,
		Now:             now,
	})

	require.NoError(t, response.Err)
	assert.False(t, response.Partial)
	assert.Equal(t, []Result{{Path: want}}, response.Results)
}

func TestSearchLocalQueryAppliesFolderType(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	want := filepath.Join(root, "reports")
	require.NoError(t, os.Mkdir(want, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "reports.txt"), nil, 0o600))
	query, err := ParseQuery("report type:folder")
	require.NoError(t, err)

	response := SearchLocalQuery(context.Background(), root, query, testLocalOptions())

	require.NoError(t, response.Err)
	assert.Equal(t, []Result{{Path: want, Directory: true}}, response.Results)
}

func TestSearchLocalQueryFindsCaseInsensitiveContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	want := filepath.Join(root, "notes.txt")
	require.NoError(t, os.WriteFile(want, []byte("The NET REVENUE rose."), 0o600))
	query, err := ParseQuery(`content:"net revenue"`)
	require.NoError(t, err)

	response := SearchLocalQuery(context.Background(), root, query, testLocalOptions())

	require.NoError(t, response.Err)
	assert.Equal(t, []Result{{Path: want}}, response.Results)
}

func TestSearchLocalQuerySkipsBinaryAndBytesPastCap(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "binary.txt"), []byte("needle\x00text"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "late.txt"), []byte("12345678needle"), 0o600))
	query, err := ParseQuery(`content:"needle"`)
	require.NoError(t, err)
	options := testLocalOptions()
	options.MaxContentBytes = 8

	response := SearchLocalQuery(context.Background(), root, query, options)

	require.NoError(t, response.Err)
	assert.Empty(t, response.Results)
}

func TestSearchLocalQueryDoesNotFollowFileSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks commonly requires elevated Windows privileges")
	}
	t.Parallel()

	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	require.NoError(t, os.WriteFile(outside, []byte("needle"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "linked.txt")))
	query, err := ParseQuery(`content:"needle"`)
	require.NoError(t, err)

	response := SearchLocalQuery(context.Background(), root, query, testLocalOptions())

	require.NoError(t, response.Err)
	assert.Empty(t, response.Results)
}

func TestSearchLocalQueryHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	query, err := ParseQuery(`content:"needle"`)
	require.NoError(t, err)

	response := SearchLocalQuery(ctx, t.TempDir(), query, testLocalOptions())

	require.ErrorIs(t, response.Err, context.Canceled)
	assert.False(t, response.Partial)
}

func TestSearchLocalQueryReportsResultAndVisitLimits(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, name := range []string{"match-a", "match-b", "match-c"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), nil, 0o600))
	}
	query, err := ParseQuery("match")
	require.NoError(t, err)
	options := testLocalOptions()
	options.ResultLimit = 2

	response := SearchLocalQuery(context.Background(), root, query, options)

	require.NoError(t, response.Err)
	assert.True(t, response.Partial)
	assert.Contains(t, response.Notice, "result limit")
	assert.Len(t, response.Results, 2)

	options.ResultLimit = 10
	options.VisitLimit = 1
	response = SearchLocalQuery(context.Background(), root, Query{Terms: []string{"missing"}}, options)
	require.NoError(t, response.Err)
	assert.True(t, response.Partial)
	assert.Contains(t, response.Notice, "visit limit")
}

func TestSearchLocalQueryPlainCompatibility(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	want := filepath.Join(root, "Quarterly-REPORT.txt")
	require.NoError(t, os.WriteFile(want, nil, 0o600))
	query, err := ParseQuery("report")
	require.NoError(t, err)

	response := SearchLocalQuery(context.Background(), root, query, testLocalOptions())

	require.NoError(t, response.Err)
	assert.Equal(t, []Result{{Path: want}}, response.Results)
}

func testLocalOptions() LocalSearchOptions {
	return LocalSearchOptions{
		ResultLimit:     10,
		VisitLimit:      100,
		MaxContentBytes: 1024,
		Now:             time.Now(),
	}
}
