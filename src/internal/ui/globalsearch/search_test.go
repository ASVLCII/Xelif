package globalsearch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEverythingCSV(t *testing.T) {
	t.Parallel()

	input := strings.NewReader("\"C:\\Docs\\report, final.txt\",A\r\n\"C:\\Projects\",D\r\n")

	results, err := parseEverythingCSV(input, 10)

	require.NoError(t, err)
	assert.Equal(t, []Result{
		{Path: `C:\Docs\report, final.txt`},
		{Path: `C:\Projects`, Directory: true},
	}, results)
}

func TestSearchLocalFindsNestedNamesCaseInsensitively(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	require.NoError(t, os.Mkdir(nested, 0o755))
	wantPath := filepath.Join(nested, "Quarterly-REPORT.txt")
	require.NoError(t, os.WriteFile(wantPath, []byte("test"), 0o600))

	results, partial, err := searchLocal(context.Background(), root, "report", 10, 100)

	require.NoError(t, err)
	assert.False(t, partial)
	assert.Equal(t, []Result{{Path: wantPath}}, results)
}

func TestSearchLocalStopsAtResultLimit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, name := range []string{"match-a", "match-b", "match-c"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), nil, 0o600))
	}

	results, partial, err := searchLocal(context.Background(), root, "match", 2, 100)

	require.NoError(t, err)
	assert.True(t, partial)
	assert.Len(t, results, 2)
}

func TestSearchLocalStopsAtVisitLimitWithoutMatches(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, name := range []string{"a", "b", "c"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), nil, 0o600))
	}

	results, partial, err := searchLocal(context.Background(), root, "missing", 10, 2)

	require.NoError(t, err)
	assert.True(t, partial)
	assert.Empty(t, results)
}

func TestSearchLocalHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results, partial, err := searchLocal(ctx, t.TempDir(), "anything", 10, 100)

	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, partial)
	assert.Empty(t, results)
}

func TestBuildEverythingArgsUsesBoundedLiteralRegex(t *testing.T) {
	t.Parallel()

	args := buildEverythingArgs(`-report (final).*`, 25)

	want := []string{
		"-regex", `-report \(final\)\.\*`,
		"-n", "25",
		"-full-path-and-name",
		"-attributes",
		"-csv",
		"-no-header",
		"-timeout", "400",
	}
	if !reflect.DeepEqual(want, args) {
		t.Fatalf("buildEverythingArgs() = %#v, want %#v", args, want)
	}
}

func TestSearchEverythingReturnsParsedResults(t *testing.T) {
	t.Parallel()

	runner := func(_ context.Context, executable string, args ...string) ([]byte, error) {
		assert.Equal(t, `C:\Tools\es.exe`, executable)
		assert.Equal(t, buildEverythingArgs("report", 10), args)
		return []byte("\"C:\\Docs\\report.txt\",A\r\n\"C:\\Reports\",D\r\n"), nil
	}

	results, err := searchEverything(
		context.Background(), `C:\Tools\es.exe`, "report", 10, runner,
	)

	require.NoError(t, err)
	assert.Equal(t, []Result{
		{Path: `C:\Docs\report.txt`},
		{Path: `C:\Reports`, Directory: true},
	}, results)
}

func TestSearchFuncUsesEverythingWhenAvailable(t *testing.T) {
	t.Parallel()

	runner := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("\"C:\\Docs\\report.txt\",A\r\n"), nil
	}
	search := newSearchFunc(`C:\Tools\es.exe`, runner)

	response := search(context.Background(), "report", t.TempDir(), 10)

	require.NoError(t, response.Err)
	assert.Equal(t, ProviderEverything, response.Provider)
	assert.False(t, response.Partial)
	assert.Empty(t, response.Notice)
	assert.Equal(t, []Result{{Path: `C:\Docs\report.txt`}}, response.Results)
}

func TestSearchFuncUsesLocalProviderForFilteredQuery(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	wantPath := filepath.Join(root, "report.txt")
	require.NoError(t, os.WriteFile(wantPath, nil, 0o600))
	runnerCalled := false
	runner := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		runnerCalled = true
		return nil, errors.New("Everything must not receive unsupported filters")
	}
	search := newSearchFunc(`C:\Tools\es.exe`, runner)

	response := search(context.Background(), "report type:file ext:txt", root, 10)

	require.NoError(t, response.Err)
	assert.False(t, runnerCalled)
	assert.Equal(t, ProviderLocal, response.Provider)
	assert.Contains(t, response.Notice, "local filtering")
	assert.Equal(t, []Result{{Path: wantPath}}, response.Results)
}

func TestSearchFuncFallsBackLocallyWhenEverythingIsMissing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	wantPath := filepath.Join(root, "needle.txt")
	require.NoError(t, os.WriteFile(wantPath, nil, 0o600))
	search := newSearchFunc("", nil)

	response := search(context.Background(), "needle", root, 10)

	require.NoError(t, response.Err)
	assert.Equal(t, ProviderLocal, response.Provider)
	assert.Contains(t, response.Notice, "Everything CLI unavailable")
	assert.Equal(t, []Result{{Path: wantPath}}, response.Results)
}

func TestSearchFuncFallsBackLocallyWhenEverythingFails(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	wantPath := filepath.Join(root, "needle.txt")
	require.NoError(t, os.WriteFile(wantPath, nil, 0o600))
	runner := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return nil, errors.New("Everything IPC unavailable")
	}
	search := newSearchFunc(`C:\Tools\es.exe`, runner)

	response := search(context.Background(), "needle", root, 10)

	require.NoError(t, response.Err)
	assert.Equal(t, ProviderLocal, response.Provider)
	assert.Contains(t, response.Notice, "Everything search failed")
	assert.Equal(t, []Result{{Path: wantPath}}, response.Results)
}

func TestSearchFuncDoesNotFallBackAfterCancellation(t *testing.T) {
	t.Parallel()

	runner := func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
		return nil, ctx.Err()
	}
	search := newSearchFunc(`C:\Tools\es.exe`, runner)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	response := search(ctx, "needle", t.TempDir(), 10)

	require.ErrorIs(t, response.Err, context.Canceled)
	assert.Equal(t, ProviderEverything, response.Provider)
	assert.Empty(t, response.Notice)
}

func TestNewDefaultSearchFuncReturnsSearcher(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, NewDefaultSearchFunc())
}
