package globalsearch

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	goosWindows = "windows"
	goosDarwin  = "darwin"
	goosLinux   = "linux"

	esExecutable        = "es.exe"
	spotlightExecutable = "/usr/bin/mdfind"
	plocateExecutable   = "plocate"
)

var (
	ErrUnsupportedIndexedQuery = errors.New("query is not supported by this indexed provider")
	ErrIndexedUnavailable      = errors.New("indexed search unavailable")
)

type indexedCommandResult struct {
	Output    []byte
	Stderr    string
	Truncated bool
	Err       error
}

type indexedCommandRunner func(context.Context, string, []string, int) indexedCommandResult

// NewProviderSearchFunc searches an explicitly selected scope. It uses only
// existing services/tools and never installs packages, starts an indexer, or
// falls back from an unavailable index to a smaller filesystem scope.
func NewProviderSearchFunc() ProviderSearchFunc {
	return newProviderSearchFunc(runtime.GOOS, exec.LookPath, runIndexedCommand, searchEverythingDirect)
}

func newProviderSearchFunc(
	goos string,
	lookPath func(string) (string, error),
	run indexedCommandRunner,
	direct directSearchFunc,
) ProviderSearchFunc {
	return func(ctx context.Context, request SearchRequest) Response {
		return providerSearch(ctx, goos, lookPath, run, direct, request)
	}
}

func providerSearch(
	ctx context.Context,
	goos string,
	lookPath func(string) (string, error),
	run indexedCommandRunner,
	direct directSearchFunc,
	request SearchRequest,
) Response {
	request.Scope = normalizeScope(request.Scope)
	request.Limit = boundedEverythingLimit(request.Limit)
	identity, capabilities := providerDescription(goos, request.Scope)
	finish := func(response Response) Response {
		response.Provider, response.Identity = identity.Name, identity
		response.Capabilities = capabilities
		response.RequestedScope, response.Root = request.Scope, request.Root
		response.Generation = request.Generation
		return response
	}
	if err := ctx.Err(); err != nil {
		return finish(Response{Err: err})
	}
	parsed, err := ParseQuery(request.Query)
	if err != nil {
		return finish(Response{Err: err})
	}
	if request.Scope == ScopeFolder {
		return finish(searchFolderScope(ctx, request, parsed))
	}
	if !indexedSupportsQuery(parsed, goos) {
		return finish(unsupportedIndexedResponse(identity.Name))
	}
	switch goos {
	case goosWindows:
		return finish(searchWindowsIndex(ctx, request, lookPath, direct))
	case goosDarwin, goosLinux:
		return finish(searchUnixIndex(ctx, goos, identity.Name, request, parsed, lookPath, run))
	default:
		return finish(unavailableIndexedResponse(
			identity.Name,
			"No indexed provider for this OS; press Ctrl+S for folder search",
		))
	}
}

func searchFolderScope(ctx context.Context, request SearchRequest, parsed Query) Response {
	if strings.TrimSpace(request.Root) == "" {
		return Response{Err: errors.New("choose a folder before using folder search")}
	}
	response := SearchLocalQuery(ctx, request.Root, parsed, LocalSearchOptions{ResultLimit: request.Limit})
	response.Truncated = response.Partial
	if errors.Is(response.Err, context.DeadlineExceeded) {
		response.Partial = true
		response.Notice = appendNotice(
			response.Notice,
			"Folder search timed out; refine the query or choose a smaller folder",
		)
	}
	return response
}

func searchWindowsIndex(
	ctx context.Context,
	request SearchRequest,
	lookPath func(string) (string, error),
	direct directSearchFunc,
) Response {
	esPath, _ := lookPath(esExecutable)
	search := withDirectEverything(direct, newSearchFunc(esPath, runEverythingCommand))
	return search(ctx, request.Query, request.Root, request.Limit)
}

func searchUnixIndex(
	ctx context.Context,
	goos string,
	provider string,
	request SearchRequest,
	parsed Query,
	lookPath func(string) (string, error),
	run indexedCommandRunner,
) Response {
	path, notice := unixIndexExecutable(goos, lookPath)
	if notice != "" {
		return unavailableIndexedResponse(provider, notice)
	}
	args := buildPlocateArgs(parsed, request.Limit)
	if goos == goosDarwin {
		args = buildSpotlightArgs(parsed)
	}
	result := run(ctx, path, args, request.Limit)
	if ctx.Err() != nil {
		return Response{Err: ctx.Err()}
	}
	// plocate uses 1 for both no matches and errors. Empty stdout AND
	// empty stderr is the normal no-match case, not an unavailable DB.
	if result.Err != nil && (goos != goosLinux || !isNoPlocateMatches(result)) {
		return unavailableIndexedResponse(provider, unixIndexFailureNotice(goos))
	}
	return unixIndexResponse(goos, result, request.Limit)
}

func unixIndexExecutable(goos string, lookPath func(string) (string, error)) (string, string) {
	executable := spotlightExecutable
	if goos == goosLinux {
		executable = plocateExecutable
	}
	path, err := lookPath(executable)
	if err != nil || path == "" {
		if goos == goosLinux {
			return "", "plocate is not installed; press Ctrl+S for folder search (nothing was installed)"
		}
		return "", "Spotlight tool unavailable; press Ctrl+S for folder search"
	}
	return path, ""
}

func unixIndexFailureNotice(goos string) string {
	if goos == goosLinux {
		return "plocate query failed; check its existing readable database, or press Ctrl+S for folder search"
	}
	return "Spotlight query failed; check existing Spotlight indexing/privacy settings, or press Ctrl+S for folder search"
}

func unixIndexResponse(goos string, result indexedCommandResult, limit int) Response {
	results, truncated, parseErr := parseNULResults(result.Output, limit)
	response := Response{
		Results:   results,
		Err:       parseErr,
		Truncated: result.Truncated || truncated,
		Notice:    "Index coverage/freshness is controlled by the system",
	}
	response.Partial = response.Truncated
	if goos == goosLinux {
		response.Notice += "; plocate case matching uses the current locale, not full Unicode folding"
	}
	if response.Truncated {
		response.Notice = appendNotice(response.Notice, "Result/output limit reached; refine your search")
	}
	if parseErr != nil {
		response.Notice = appendNotice(response.Notice, "Invalid indexed output; press Ctrl+S for folder search")
	}
	return response
}

func normalizeScope(scope Scope) Scope {
	if scope == ScopeFolder {
		return ScopeFolder
	}
	return ScopeIndexed
}

func providerDescription(goos string, scope Scope) (ProviderIdentity, Capabilities) {
	if normalizeScope(scope) == ScopeFolder {
		return ProviderIdentity{
				ID:   string(ScopeFolder),
				Name: ProviderLocal,
			}, Capabilities{
				NameTerms: true,
				MultiTerm: true,
				Filters:   true,
				Content:   true,
			}
	}
	capabilities := Capabilities{NameTerms: true, MultiTerm: goos != goosWindows, Indexed: true}
	switch goos {
	case goosWindows:
		return ProviderIdentity{ID: "everything", Name: ProviderEverything}, capabilities
	case goosDarwin:
		return ProviderIdentity{ID: "spotlight", Name: ProviderSpotlight}, capabilities
	case goosLinux:
		return ProviderIdentity{ID: "plocate", Name: ProviderPlocate}, capabilities
	default:
		return ProviderIdentity{ID: "unavailable", Name: "Indexed"}, Capabilities{Indexed: true}
	}
}

func indexedSupportsQuery(query Query, goos string) bool {
	if goos == goosWindows {
		return EverythingSupportsQuery(query)
	}
	return len(query.Terms) > 0 && query.Type == EntryTypeAny && len(query.Extensions) == 0 &&
		len(query.Sizes) == 0 && query.ModifiedWithin == 0 && query.Content == "" &&
		!strings.Contains(strings.Join(query.Terms, ""), "\x00")
}

func unsupportedIndexedResponse(provider string) Response {
	return Response{
		Provider: provider,
		Err:      ErrUnsupportedIndexedQuery,
		Notice:   "This indexed provider cannot preserve these filters; press Ctrl+S to explicitly search the current folder",
	}
}

func unavailableIndexedResponse(provider, notice string) Response {
	return Response{Provider: provider, Err: ErrIndexedUnavailable, Notice: notice}
}

// Apple File Metadata Query Expression Syntax documents quoted values, escaped
// quotes, * / ? wildcards and &&. Backslash makes user punctuation literal:
// https://developer.apple.com/library/archive/documentation/Carbon/Conceptual/SpotlightQuery/Concepts/QueryFormat.html
func buildSpotlightArgs(query Query) []string {
	terms := make([]string, 0, len(query.Terms))
	for _, term := range query.Terms {
		var literal strings.Builder
		for _, r := range term {
			if strings.ContainsRune(`\"'*?`, r) {
				literal.WriteByte('\\')
			}
			literal.WriteRune(r)
		}
		terms = append(terms, `kMDItemFSName ==[c] "*`+literal.String()+`*"`)
	}
	return []string{"-0", strings.Join(terms, " && ")}
}

// plocate's primary manual documents ANDed patterns, glob backslash escaping,
// -- to end options, -0, -i, -b and the match limit. Explicit *...* preserves
// substring semantics even when user text contains glob punctuation.
// https://plocate.sesse.net/plocate.1.html
func buildPlocateArgs(query Query, limit int) []string {
	args := []string{"-0", "-i", "-b", "-l", strconv.Itoa(boundedEverythingLimit(limit) + 1), "--"}
	for _, term := range query.Terms {
		var literal strings.Builder
		literal.WriteByte('*')
		for _, r := range term {
			if strings.ContainsRune(`\*?[]`, r) {
				literal.WriteByte('\\')
			}
			literal.WriteRune(r)
		}
		literal.WriteByte('*')
		args = append(args, literal.String())
	}
	return args
}

func isNoPlocateMatches(result indexedCommandResult) bool {
	var exitErr *exec.ExitError
	return errors.As(result.Err, &exitErr) && exitErr.ExitCode() == 1 && len(result.Output) == 0 &&
		strings.TrimSpace(result.Stderr) == ""
}

func parseNULResults(data []byte, limit int) ([]Result, bool, error) {
	limit = boundedEverythingLimit(limit)
	results := make([]Result, 0, min(limit, bytes.Count(data, []byte{0})))
	for len(data) > 0 {
		index := bytes.IndexByte(data, 0)
		if index < 0 {
			return results, false, errors.New("indexed search returned an unterminated path")
		}
		path := string(data[:index])
		if !utf8.ValidString(path) {
			return results, false, errors.New("indexed path is not UTF-8; unsupported filename or locale")
		}
		data = data[index+1:]
		if path == "" {
			continue
		}
		if len(path) > indexedPathLimit {
			return results, true, errors.New("indexed search returned an oversized path")
		}
		if !strings.HasPrefix(path, "/") {
			return results, false, errors.New("indexed search returned a non-absolute path")
		}
		if len(results) >= limit {
			return results, true, nil
		}
		// No eager stat fan-out. Revealing the selected path resolves its kind.
		results = append(results, Result{Path: path})
	}
	return results, false, nil
}
