package globalsearch

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const localVisitLimit = 50_000

// Bound optional Windows transport attempts without changing the chosen scope.
const everythingAttemptTimeout = 500 * time.Millisecond

var errSearchLimitReached = errors.New("search limit reached")

type commandRunner func(context.Context, string, ...string) ([]byte, error)

// NewDefaultSearchFunc retains the original injectable interface. The default
// scope is the OS's existing index; it never silently narrows to a folder.
func NewDefaultSearchFunc() SearchFunc {
	search := NewProviderSearchFunc()
	return func(ctx context.Context, query, root string, limit int) Response {
		return search(ctx, SearchRequest{Query: query, Root: root, Scope: ScopeIndexed, Limit: limit})
	}
}

type directSearchFunc func(context.Context, string, int) ([]Result, error)

func withDirectEverything(direct directSearchFunc, fallback SearchFunc) SearchFunc {
	return func(ctx context.Context, query, root string, limit int) Response {
		parsed, err := ParseQuery(query)
		if err != nil {
			return Response{Provider: ProviderEverything, Err: err}
		}
		if !EverythingSupportsQuery(parsed) {
			return unsupportedIndexedResponse(ProviderEverything)
		}
		attempt, cancel := context.WithTimeout(ctx, everythingAttemptTimeout)
		results, directErr := direct(attempt, parsed.Terms[0], limit)
		cancel()
		if ctx.Err() != nil {
			return Response{Provider: ProviderEverything, Err: ctx.Err()}
		}
		if directErr == nil {
			return everythingResponse(results, limit)
		}
		if errors.Is(directErr, context.Canceled) {
			return Response{Provider: ProviderEverything, Err: directErr}
		}
		if fallback != nil {
			return fallback(ctx, query, root, limit)
		}
		return unavailableIndexedResponse(
			ProviderEverything,
			"Open the existing Everything app, or press Ctrl+S for folder search",
		)
	}
}

func newSearchFunc(esPath string, run commandRunner) SearchFunc {
	return func(ctx context.Context, query string, root string, limit int) Response {
		parsed, err := ParseQuery(query)
		if err != nil {
			return Response{Provider: ProviderEverything, Err: err}
		}
		if !EverythingSupportsQuery(parsed) {
			return unsupportedIndexedResponse(ProviderEverything)
		}

		if esPath != "" && EverythingSupportsQuery(parsed) {
			attempt, cancel := context.WithTimeout(ctx, everythingAttemptTimeout)
			results, everythingErr := searchEverything(attempt, esPath, parsed.Terms[0], limit, run)
			cancel()
			if ctx.Err() != nil {
				return Response{Provider: ProviderEverything, Err: ctx.Err()}
			}
			if everythingErr == nil {
				return everythingResponse(results, limit)
			}
			if errors.Is(everythingErr, context.Canceled) {
				return Response{Provider: ProviderEverything, Err: everythingErr}
			}
			return unavailableIndexedResponse(
				ProviderEverything,
				"Everything query failed; open the existing Everything app, or press Ctrl+S for folder search",
			)
		}
		return unavailableIndexedResponse(
			ProviderEverything,
			"Everything IPC and optional es.exe unavailable; open the existing Everything app, or press Ctrl+S for folder search",
		)
	}
}

func everythingResponse(results []Result, limit int) Response {
	limit = boundedEverythingLimit(limit)
	response := Response{Results: results[:min(len(results), limit)], Provider: ProviderEverything}
	if len(results) >= limit {
		response.Partial = true
		response.Truncated = true
		response.Notice = "Result limit reached; refine your search"
	}
	return response
}

func buildEverythingArgs(query string, limit int) []string {
	limit = boundedEverythingLimit(limit)
	return []string{
		"-regex", regexp.QuoteMeta(query),
		"-n", strconv.Itoa(limit),
		"-full-path-and-name",
		"-attributes",
		"-csv",
		"-no-header",
		"-timeout", "400",
	}
}

func searchEverything(
	ctx context.Context,
	executable string,
	query string,
	limit int,
	run commandRunner,
) ([]Result, error) {
	output, err := run(ctx, executable, buildEverythingArgs(query, limit)...)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	return parseEverythingCSV(bytes.NewReader(output), limit)
}

func searchLocal(
	ctx context.Context,
	root string,
	query string,
	resultLimit int,
	visitLimit int,
) ([]Result, bool, error) {
	response := SearchLocalQuery(ctx, root, Query{Terms: []string{query}}, LocalSearchOptions{
		ResultLimit: resultLimit,
		VisitLimit:  visitLimit,
	})
	return response.Results, response.Partial, response.Err
}

func boundedEverythingLimit(limit int) int {
	if limit <= 0 {
		return defaultResultLimit
	}
	return min(limit, localVisitLimit)
}

func parseEverythingCSV(input io.Reader, limit int) ([]Result, error) {
	limit = boundedEverythingLimit(limit)
	reader := csv.NewReader(input)
	reader.FieldsPerRecord = -1

	results := make([]Result, 0)
	for len(results) < limit {
		record, err := reader.Read()
		if err == io.EOF {
			return results, nil
		}
		if err != nil {
			return nil, err
		}
		if len(record) == 0 || record[0] == "" {
			continue
		}

		attributes := ""
		if len(record) > 1 {
			attributes = strings.ToUpper(record[1])
		}
		results = append(results, Result{
			Path:      record[0],
			Directory: strings.Contains(attributes, "D"),
		})
	}
	return results, nil
}
