package globalsearch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultMaxContentBytes int64 = 1024 * 1024
	contentReadBufferBytes int64 = 32 * 1024
)

// LocalSearchOptions bounds filesystem and content work for a local query.
type LocalSearchOptions struct {
	ResultLimit     int
	VisitLimit      int
	MaxContentBytes int64
	Now             time.Time
}

type localSearchWalker struct {
	ctx           context.Context
	root          string
	query         Query
	options       LocalSearchOptions
	response      *Response
	visited       int
	skippedErrors int
	stopReason    string
}

// SearchLocalQuery performs a bounded local search without following symlinks.
// Content matching reads regular files only, skips NUL-containing data, and is
// case-insensitive.
func SearchLocalQuery(ctx context.Context, root string, query Query, options LocalSearchOptions) Response {
	response := Response{Provider: ProviderLocal}
	if err := ctx.Err(); err != nil {
		response.Err = err
		return response
	}
	options = normalizeLocalSearchOptions(options)
	walker := localSearchWalker{
		ctx: ctx, root: root, query: query, options: options, response: &response,
	}
	//nolint:gosec // G703: The root is the user-selected search scope, not a sandbox boundary; symlinks are not followed.
	err := filepath.WalkDir(root, walker.visit)

	switch {
	case errors.Is(err, errSearchLimitReached):
		response.Partial = true
		response.Notice = walker.stopReason
	case err != nil:
		response.Err = err
	}
	if walker.skippedErrors > 0 {
		response.Partial = true
		response.Notice = appendNotice(
			response.Notice,
			fmt.Sprintf("skipped %d unreadable path(s)", walker.skippedErrors),
		)
	}
	return response
}

func (walker *localSearchWalker) visit(path string, entry fs.DirEntry, walkErr error) error {
	if err := walker.ctx.Err(); err != nil {
		return err
	}
	if walkErr != nil {
		if path == walker.root {
			return walkErr
		}
		walker.skippedErrors++
		return nil
	}
	if path == walker.root {
		return nil
	}

	walker.visited++
	if entry.Type()&fs.ModeSymlink != 0 {
		return walker.enforceVisitLimit()
	}
	// Most selective name queries reject entries using only directory data.
	// Do this before Info, which may require an extra filesystem request for
	// every entry (especially expensive on remote or large directories). Returning
	// nil for nonmatching directories still lets WalkDir search their children.
	if !nameMatches(entry.Name(), walker.query.Terms) {
		return walker.enforceVisitLimit()
	}
	info, err := entry.Info()
	if err != nil {
		walker.skippedErrors++
		return walker.enforceVisitLimit()
	}
	if err := walker.addMatch(path, entry, info); err != nil {
		return err
	}
	return walker.enforceVisitLimit()
}

func (walker *localSearchWalker) addMatch(path string, entry fs.DirEntry, info fs.FileInfo) error {
	matched, err := localEntryMatches(
		walker.ctx,
		path,
		entry,
		info,
		walker.query,
		walker.options,
	)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		walker.skippedErrors++
		return nil
	}
	if !matched {
		return nil
	}
	walker.response.Results = append(
		walker.response.Results,
		Result{Path: path, Directory: entry.IsDir()},
	)
	if len(walker.response.Results) >= walker.options.ResultLimit {
		walker.stopReason = "result limit reached"
		return errSearchLimitReached
	}
	return nil
}

func (walker *localSearchWalker) enforceVisitLimit() error {
	if walker.visited < walker.options.VisitLimit {
		return nil
	}
	walker.stopReason = "visit limit reached"
	return errSearchLimitReached
}

func normalizeLocalSearchOptions(options LocalSearchOptions) LocalSearchOptions {
	if options.ResultLimit <= 0 {
		options.ResultLimit = defaultResultLimit
	}
	if options.VisitLimit <= 0 {
		options.VisitLimit = localVisitLimit
	}
	if options.MaxContentBytes <= 0 {
		options.MaxContentBytes = defaultMaxContentBytes
	}
	if options.Now.IsZero() {
		options.Now = time.Now()
	}
	return options
}

func localEntryMatches(
	ctx context.Context,
	path string,
	entry fs.DirEntry,
	info fs.FileInfo,
	query Query,
	options LocalSearchOptions,
) (bool, error) {
	if query.Type == EntryTypeFile && !info.Mode().IsRegular() {
		return false, nil
	}
	if query.Type == EntryTypeFolder && !entry.IsDir() {
		return false, nil
	}
	if len(query.Extensions) > 0 {
		if !info.Mode().IsRegular() || !extensionMatches(path, query.Extensions) {
			return false, nil
		}
	}
	if len(query.Sizes) > 0 {
		if !info.Mode().IsRegular() || !sizeMatches(info.Size(), query.Sizes) {
			return false, nil
		}
	}
	if query.ModifiedWithin > 0 && info.ModTime().Before(options.Now.Add(-query.ModifiedWithin)) {
		return false, nil
	}
	if query.Content == "" {
		return true, nil
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	return fileContentMatches(ctx, path, query.Content, options.MaxContentBytes)
}

func nameMatches(name string, terms []string) bool {
	name = strings.ToLower(name)
	for _, term := range terms {
		if !strings.Contains(name, strings.ToLower(term)) {
			return false
		}
	}
	return true
}

func extensionMatches(path string, extensions []string) bool {
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	for _, allowed := range extensions {
		if extension == allowed {
			return true
		}
	}
	return false
}

func sizeMatches(size int64, predicates []SizePredicate) bool {
	for _, predicate := range predicates {
		switch predicate.Comparison {
		case GreaterThan:
			if size <= predicate.Bytes {
				return false
			}
		case LessThan:
			if size >= predicate.Bytes {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func fileContentMatches(ctx context.Context, path string, content string, maxBytes int64) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	data := make([]byte, 0, min(maxBytes, contentReadBufferBytes))
	buffer := make([]byte, min(maxBytes, contentReadBufferBytes))
	remaining := maxBytes
	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		readSize := int64(len(buffer))
		if readSize > remaining {
			readSize = remaining
		}
		count, readErr := file.Read(buffer[:readSize])
		if count > 0 {
			chunk := buffer[:count]
			if bytes.IndexByte(chunk, 0) >= 0 {
				return false, nil
			}
			data = append(data, chunk...)
			remaining -= int64(count)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return false, readErr
		}
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return strings.Contains(strings.ToLower(string(data)), strings.ToLower(content)), nil
}

func appendNotice(existing string, addition string) string {
	if existing == "" {
		return addition
	}
	return existing + "; " + addition
}
