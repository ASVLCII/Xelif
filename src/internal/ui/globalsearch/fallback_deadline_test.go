package globalsearch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProviderTimeoutsLeaveLocalFallbackBudget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "needle.txt")
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	direct := func(ctx context.Context, _ string, _ int) ([]Result, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	runner := func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
		<-ctx.Done()
		// A killed CommandContext process need not return context's own error.
		return nil, errors.New("process killed")
	}
	search := withDirectEverything(direct, newSearchFunc("es.exe", runner))
	ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
	defer cancel()
	response := search(ctx, "needle", root, 10)
	require.NoError(t, response.Err)
	require.NoError(t, ctx.Err())
	require.Equal(t, ProviderLocal, response.Provider)
	require.Equal(t, []Result{{Path: path}}, response.Results)
	require.Contains(t, response.Notice, "current folder")
}

func TestCallerDeadlineDoesNotRunFallback(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	called := false
	search := withDirectEverything(func(ctx context.Context, _ string, _ int) ([]Result, error) {
		return nil, ctx.Err()
	}, func(context.Context, string, string, int) Response {
		called = true
		return Response{}
	})
	response := search(ctx, "needle", t.TempDir(), 10)
	require.ErrorIs(t, response.Err, context.DeadlineExceeded)
	require.False(t, called)
}

func TestDirectAttemptDeadlineFallsBackWhenCallerIsAlive(t *testing.T) {
	t.Parallel()
	search := withDirectEverything(func(context.Context, string, int) ([]Result, error) {
		return nil, context.DeadlineExceeded
	}, func(ctx context.Context, _, _ string, _ int) Response {
		require.NoError(t, ctx.Err())
		return Response{Provider: ProviderLocal}
	})
	response := search(context.Background(), "needle", t.TempDir(), 10)
	require.NoError(t, response.Err)
	require.Equal(t, ProviderLocal, response.Provider)
}

func TestKilledCLIReportsCallerCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	search := newSearchFunc("es.exe", func(context.Context, string, ...string) ([]byte, error) {
		cancel()
		return nil, errors.New("process killed")
	})
	response := search(ctx, "needle", t.TempDir(), 10)
	require.ErrorIs(t, response.Err, context.Canceled)
	require.Equal(t, ProviderEverything, response.Provider)
	require.Empty(t, response.Results)
}

func TestEverythingCSVHonorsClientResultCap(t *testing.T) {
	t.Parallel()
	results, err := parseEverythingCSV(strings.NewReader("one,A\ntwo,A\nthree,A\n"), 2)
	require.NoError(t, err)
	require.Equal(t, []Result{{Path: "one"}, {Path: "two"}}, results)
	require.Equal(t, defaultResultLimit, boundedEverythingLimit(0))
	require.Equal(t, defaultResultLimit, boundedEverythingLimit(-1))
	require.Equal(t, localVisitLimit, boundedEverythingLimit(localVisitLimit+1))
}

func TestEverythingResultCapIsVisibleForBothProviders(t *testing.T) {
	t.Parallel()
	direct := withDirectEverything(func(context.Context, string, int) ([]Result, error) {
		return []Result{{Path: "one"}, {Path: "two"}}, nil
	}, nil)
	cli := newSearchFunc("es.exe", func(context.Context, string, ...string) ([]byte, error) {
		return []byte("one,A\ntwo,A\n"), nil
	})
	for _, search := range []SearchFunc{direct, cli} {
		response := search(context.Background(), "one", t.TempDir(), 1)
		require.NoError(t, response.Err)
		require.Len(t, response.Results, 1)
		require.True(t, response.Partial)
		require.Contains(t, response.Notice, "limit reached")
	}
}
