package globalsearch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexedProviderTranslatesLiteralQuotedUnicodeQueries(t *testing.T) {
	t.Parallel()
	parsed, err := ParseQuery(`"say \"hello\" \\ 路線*?[x]" -needle`)
	require.NoError(t, err)
	assert.Equal(
		t,
		[]string{"-0", `kMDItemFSName ==[c] "*say \"hello\" \\ 路線\*\?[x]*" && kMDItemFSName ==[c] "*-needle*"`},
		buildSpotlightArgs(parsed),
	)
	assert.Equal(
		t,
		[]string{"-0", "-i", "-b", "-l", "8", "--", `*say "hello" \\ 路線\*\?\[x\]*`, "*-needle*"},
		buildPlocateArgs(parsed, 7),
	)
}

func TestNativeProvidersKeepRequestedScopeAndSkipAllMetadataFanout(t *testing.T) {
	t.Parallel()
	for _, goos := range []string{"darwin", "linux"} {
		t.Run(goos, func(t *testing.T) {
			t.Parallel()
			calls := 0
			search := newProviderSearchFunc(goos, func(tool string) (string, error) {
				if goos == "darwin" {
					assert.Equal(t, "/usr/bin/mdfind", tool)
				} else {
					assert.Equal(t, "plocate", tool)
				}
				return tool, nil
			}, func(_ context.Context, executable string, args []string, limit int) indexedCommandResult {
				calls++
				assert.Equal(t, 2, limit)
				assert.NotContains(t, args, "-onlyin", "indexed scope must not silently narrow to the active folder")
				return indexedCommandResult{Output: []byte("/outside/報告.txt\x00/outside/name\nwith newline\x00")}
			}, nil)
			response := search(
				context.Background(),
				SearchRequest{
					Query:      `"報告 final"`,
					Scope:      ScopeIndexed,
					Root:       "/active/folder",
					Limit:      2,
					Generation: 42,
				},
			)
			require.NoError(t, response.Err)
			assert.Equal(t, ScopeIndexed, response.RequestedScope)
			assert.Equal(t, "/active/folder", response.Root)
			assert.Equal(t, uint64(42), response.Generation)
			assert.True(t, response.Capabilities.Indexed)
			assert.False(t, response.Capabilities.Content)
			assert.NotEmpty(t, response.Identity.ID)
			assert.Equal(
				t,
				[]Result{{Path: "/outside/報告.txt"}, {Path: "/outside/name\nwith newline"}},
				response.Results,
			)
			assert.False(t, response.Truncated, "exactly the result cap is complete without an extra record")
			assert.Equal(t, 1, calls)
		})
	}
}

func TestAllIndexedProvidersRequireExplicitFolderForAdvancedSyntax(t *testing.T) {
	t.Parallel()
	queries := []string{"report type:file", "ext:txt", "size:>1k", "modified:<7d", `content:"needle"`}
	for _, goos := range []string{"windows", "darwin", "linux"} {
		for _, query := range queries {
			t.Run(goos+query, func(t *testing.T) {
				t.Parallel()
				search := newProviderSearchFunc(
					goos,
					func(string) (string, error) { t.Fatal("unsupported syntax must not probe/run tools"); return "", nil },
					nil,
					nil,
				)
				response := search(
					context.Background(),
					SearchRequest{Query: query, Scope: ScopeIndexed, Root: t.TempDir(), Generation: 9},
				)
				require.ErrorIs(t, response.Err, ErrUnsupportedIndexedQuery)
				assert.Equal(t, ScopeIndexed, response.RequestedScope)
				assert.NotEqual(t, ProviderLocal, response.Provider)
				assert.Contains(t, response.Notice, "Ctrl+S")
				assert.Empty(t, response.Results)
			})
		}
	}
}

func TestUnavailableIndexNeverWalksCurrentFolder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "needle.txt"), nil, 0o600))
	for _, goos := range []string{"linux", "darwin", "plan9"} {
		search := newProviderSearchFunc(goos, func(string) (string, error) { return "", os.ErrNotExist }, nil, nil)
		response := search(context.Background(), SearchRequest{Query: "needle", Root: root, Scope: ScopeIndexed})
		require.ErrorIs(t, response.Err, ErrIndexedUnavailable)
		assert.Empty(t, response.Results)
		assert.Equal(t, ScopeIndexed, response.RequestedScope)
		assert.Contains(t, response.Notice, "Ctrl+S")
	}
}

func TestExplicitFolderProviderUsesSharedParserAndBounds(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"Report-A.txt", "report-B.txt", "report-C.pdf"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte("needle"), 0o600))
	}
	search := newProviderSearchFunc(
		"linux",
		func(string) (string, error) { t.Fatal("folder mode must not run/probe native tools"); return "", nil },
		nil,
		nil,
	)
	response := search(
		context.Background(),
		SearchRequest{
			Query:      `report ext:txt content:"needle"`,
			Scope:      ScopeFolder,
			Root:       root,
			Limit:      1,
			Generation: 8,
		},
	)
	require.NoError(t, response.Err)
	assert.Equal(t, ProviderLocal, response.Provider)
	assert.Equal(t, ScopeFolder, response.RequestedScope)
	assert.True(t, response.Capabilities.Filters)
	assert.True(t, response.Capabilities.Content)
	assert.True(t, response.Truncated)
	assert.Len(t, response.Results, 1)
	assert.Contains(t, response.Notice, "limit reached")
	response = search(context.Background(), SearchRequest{Query: "needle", Scope: ScopeFolder})
	require.ErrorContains(t, response.Err, "choose a folder")
}

func TestNativeOutputParsingIsNULBoundedAndRejectsInvalidPaths(t *testing.T) {
	t.Parallel()
	results, truncated, err := parseNULResults([]byte("/a\x00/b\x00/c\x00"), 2)
	require.NoError(t, err)
	assert.True(t, truncated)
	assert.Equal(t, []Result{{Path: "/a"}, {Path: "/b"}}, results)
	for _, data := range []string{"/unterminated", "relative\x00", "/" + strings.Repeat("x", indexedPathLimit) + "\x00"} {
		_, _, err = parseNULResults([]byte(data), 2)
		require.Error(t, err)
	}
}

func TestNativeFailureAndSafetyLimitNoticesAreActionable(t *testing.T) {
	t.Parallel()
	for _, goos := range []string{"linux", "darwin"} {
		for _, fail := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/failure=%v", goos, fail), func(t *testing.T) {
				t.Parallel()
				search := newProviderSearchFunc(
					goos,
					func(tool string) (string, error) { return tool, nil },
					func(context.Context, string, []string, int) indexedCommandResult {
						if fail {
							return indexedCommandResult{Err: errors.New("database unavailable"), Stderr: "not readable"}
						}
						return indexedCommandResult{Output: []byte("/a\x00"), Truncated: true}
					},
					nil,
				)
				response := search(context.Background(), SearchRequest{Query: "needle", Scope: ScopeIndexed})
				if fail {
					require.ErrorIs(t, response.Err, ErrIndexedUnavailable)
					assert.Contains(t, response.Notice, "Ctrl+S")
				} else {
					require.NoError(t, response.Err)
					assert.True(t, response.Partial)
					assert.True(t, response.Truncated)
					assert.Contains(t, response.Notice, "limit reached")
				}
				assert.Equal(t, ScopeIndexed, response.RequestedScope)
			})
		}
	}
}

func TestQueryRejectsNULAndOversizedInput(t *testing.T) {
	t.Parallel()
	for _, query := range []string{"needle\x00ignored", strings.Repeat("x", 8193)} {
		_, err := ParseQuery(query)
		require.Error(t, err)
	}
}
