package globalsearch

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

// A query owns one pattern and one read buffer across all candidate files. KMP
// retains only the matched prefix instead of retaining/lowercasing whole files.
type contentMatcher struct {
	pattern []rune
	prefix  []int
	buffer  []byte
}

func newContentMatcher(content string) *contentMatcher {
	pattern := []rune(strings.ToLower(content))
	prefix := make([]int, len(pattern))
	matched := 0
	for index := 1; index < len(pattern); index++ {
		for matched > 0 && pattern[index] != pattern[matched] {
			matched = prefix[matched-1]
		}
		if pattern[index] == pattern[matched] {
			matched++
		}
		prefix[index] = matched
	}
	return &contentMatcher{pattern: pattern, prefix: prefix}
}

func (matcher *contentMatcher) fileMatches(ctx context.Context, path string, maxBytes int64) (bool, error) {
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
	return matcher.readerMatches(ctx, file, maxBytes)
}

func (matcher *contentMatcher) readerMatches(ctx context.Context, reader io.Reader, maxBytes int64) (bool, error) {
	// Extra space holds an incomplete UTF-8 rune across a read boundary.
	needed := int(min(maxBytes, contentReadBufferBytes)) + utf8.UTFMax
	if len(matcher.buffer) < needed {
		matcher.buffer = make([]byte, needed)
	}
	remaining := maxBytes
	state := contentMatchState{matcher: matcher, found: len(matcher.pattern) == 0}
	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		readSize := int(min(remaining, contentReadBufferBytes))
		count, readErr := reader.Read(matcher.buffer[state.carried : state.carried+readSize])
		if count > 0 {
			remaining -= int64(count)
			if state.consumeRead(count, remaining == 0 || readErr != nil) {
				return false, nil
			}
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				return false, readErr
			}
			break
		}
		if count == 0 {
			return false, io.ErrNoProgress
		}
	}
	// An EOF returned separately from its last bytes leaves a partial rune to
	// decode as replacement characters, matching strings.ToLower's behavior.
	if !state.found && state.carried > 0 {
		matcher.matchChunk(matcher.buffer[:state.carried], &state.matched, &state.found, true)
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return state.found, nil
}

type contentMatchState struct {
	matcher *contentMatcher
	carried int
	matched int
	found   bool
}

// consumeRead returns true for binary data. A match never ends the scan early:
// NUL bytes later in the bounded input must still reject the entire file.
func (state *contentMatchState) consumeRead(count int, atEOF bool) bool {
	chunk := state.matcher.buffer[:state.carried+count]
	if bytes.IndexByte(chunk, 0) >= 0 {
		return true
	}
	if state.found {
		state.carried = 0
		return false
	}
	consumed := state.matcher.matchChunk(chunk, &state.matched, &state.found, atEOF)
	state.carried = copy(state.matcher.buffer, chunk[consumed:])
	return false
}

func (matcher *contentMatcher) matchChunk(chunk []byte, matched *int, found *bool, atEOF bool) int {
	index := 0
	for index < len(chunk) {
		value, size := rune(chunk[index]), 1
		if value >= utf8.RuneSelf {
			if !atEOF && !utf8.FullRune(chunk[index:]) {
				break
			}
			value, size = utf8.DecodeRune(chunk[index:])
			value = unicode.ToLower(value)
		} else if value >= 'A' && value <= 'Z' {
			value += 'a' - 'A'
		}
		index += size
		for *matched > 0 && value != matcher.pattern[*matched] {
			*matched = matcher.prefix[*matched-1]
		}
		if value == matcher.pattern[*matched] {
			*matched++
			if *matched == len(matcher.pattern) {
				*found = true
				return len(chunk)
			}
		}
	}
	return index
}
