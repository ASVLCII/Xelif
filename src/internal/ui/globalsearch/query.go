package globalsearch

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// EntryType limits a query to files or folders.
type EntryType uint8

const (
	EntryTypeAny EntryType = iota
	EntryTypeFile
	EntryTypeFolder
)

// Comparison is the comparison used by a size predicate.
type Comparison uint8

const (
	GreaterThan Comparison = iota + 1
	LessThan
)

// SizePredicate compares a regular file's size in bytes.
type SizePredicate struct {
	Comparison Comparison
	Bytes      int64
}

// Query is the parsed, provider-independent form of a global-search query.
// Name terms are ANDed, while repeated extensions are ORed. ModifiedWithin
// represents the minimal relative syntax modified:<Nd (within the last N days).
type Query struct {
	Terms          []string
	Type           EntryType
	Extensions     []string
	Sizes          []SizePredicate
	ModifiedWithin time.Duration
	Content        string
}

type queryToken struct {
	text   string
	quoted bool
}

var sizePattern = regexp.MustCompile(`^([0-9]+)([kKmMgG]?)$`)

const (
	bytesPerKibibyte int64 = 1024
	bytesPerMebibyte       = bytesPerKibibyte * bytesPerKibibyte
	bytesPerGibibyte       = bytesPerMebibyte * bytesPerKibibyte
	hoursPerDay            = 24
)

// ParseQuery parses free-text name terms and inline type, ext, size, modified,
// and quoted content filters.
func ParseQuery(input string) (Query, error) {
	tokens, err := tokenizeQuery(input)
	if err != nil {
		return Query{}, err
	}

	var query Query
	for _, token := range tokens {
		if err := applyQueryToken(&query, token); err != nil {
			return Query{}, err
		}
	}
	return query, nil
}

func applyQueryToken(query *Query, token queryToken) error {
	key, value, hasValue := strings.Cut(token.text, ":")
	if !hasValue {
		query.Terms = append(query.Terms, token.text)
		return nil
	}

	switch strings.ToLower(key) {
	case "type":
		return applyTypeFilter(query, value)
	case "ext":
		return applyExtensionFilter(query, value)
	case "size":
		return applySizeFilter(query, value)
	case "modified":
		return applyModifiedFilter(query, value)
	case "content":
		return applyContentFilter(query, value, token.quoted)
	default:
		query.Terms = append(query.Terms, token.text)
		return nil
	}
}

func applyTypeFilter(query *Query, value string) error {
	entryType, err := parseEntryType(value)
	if err != nil {
		return err
	}
	if query.Type != EntryTypeAny && query.Type != entryType {
		return errors.New("type filter cannot select both file and folder")
	}
	query.Type = entryType
	return nil
}

func applyExtensionFilter(query *Query, value string) error {
	extension := strings.TrimPrefix(strings.ToLower(value), ".")
	if extension == "" || strings.ContainsAny(extension, `/\\`) {
		return fmt.Errorf("invalid extension filter %q", value)
	}
	query.Extensions = append(query.Extensions, extension)
	return nil
}

func applySizeFilter(query *Query, value string) error {
	predicate, err := parseSizePredicate(value)
	if err != nil {
		return err
	}
	query.Sizes = append(query.Sizes, predicate)
	return nil
}

func applyModifiedFilter(query *Query, value string) error {
	if query.ModifiedWithin != 0 {
		return errors.New("modified filter may only be specified once")
	}
	duration, err := parseModifiedWithin(value)
	if err != nil {
		return err
	}
	query.ModifiedWithin = duration
	return nil
}

func applyContentFilter(query *Query, value string, quoted bool) error {
	if !quoted || value == "" {
		return errors.New(
			`content filter must use non-empty quoted text, for example content:"needle"`,
		)
	}
	if query.Content != "" {
		return errors.New("content filter may only be specified once")
	}
	query.Content = value
	return nil
}

func parseEntryType(value string) (EntryType, error) {
	switch strings.ToLower(value) {
	case "file":
		return EntryTypeFile, nil
	case "folder":
		return EntryTypeFolder, nil
	default:
		return EntryTypeAny, fmt.Errorf("invalid type filter %q: want file or folder", value)
	}
}

func parseSizePredicate(value string) (SizePredicate, error) {
	if len(value) < 2 || (value[0] != '>' && value[0] != '<') {
		return SizePredicate{}, fmt.Errorf("invalid size filter %q: want >N or <N", value)
	}
	bytes, err := parseByteCount(value[1:])
	if err != nil {
		return SizePredicate{}, fmt.Errorf("invalid size filter %q: %w", value, err)
	}
	comparison := GreaterThan
	if value[0] == '<' {
		comparison = LessThan
	}
	return SizePredicate{Comparison: comparison, Bytes: bytes}, nil
}

func parseByteCount(value string) (int64, error) {
	matches := sizePattern.FindStringSubmatch(value)
	if matches == nil {
		return 0, errors.New("size must be an integer with an optional k, m, or g suffix")
	}
	number, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil || number <= 0 {
		return 0, errors.New("size must be a positive integer")
	}
	factor := int64(1)
	switch strings.ToLower(matches[2]) {
	case "k":
		factor = bytesPerKibibyte
	case "m":
		factor = bytesPerMebibyte
	case "g":
		factor = bytesPerGibibyte
	}
	if number > math.MaxInt64/factor {
		return 0, errors.New("size is too large")
	}
	return number * factor, nil
}

func parseModifiedWithin(value string) (time.Duration, error) {
	if !strings.HasPrefix(value, "<") || !strings.HasSuffix(strings.ToLower(value), "d") {
		return 0, fmt.Errorf("invalid modified filter %q: want <Nd", value)
	}
	days, err := strconv.ParseInt(value[1:len(value)-1], 10, 64)
	if err != nil || days <= 0 || days > int64(math.MaxInt64/(hoursPerDay*time.Hour)) {
		return 0, fmt.Errorf("invalid modified filter %q: days must be a positive integer", value)
	}
	return time.Duration(days) * hoursPerDay * time.Hour, nil
}

func tokenizeQuery(input string) ([]queryToken, error) {
	runes := []rune(input)
	tokens := make([]queryToken, 0)
	var builder strings.Builder
	quoted := false
	started := false
	tokenQuoted := false

	flush := func() {
		if !started {
			return
		}
		tokens = append(tokens, queryToken{text: builder.String(), quoted: tokenQuoted})
		builder.Reset()
		started = false
		tokenQuoted = false
	}

	for index := 0; index < len(runes); index++ {
		character := runes[index]
		if character == '"' {
			quoted = !quoted
			started = true
			tokenQuoted = true
			continue
		}
		if quoted && character == '\\' && index+1 < len(runes) &&
			(runes[index+1] == '"' || runes[index+1] == '\\') {
			index++
			builder.WriteRune(runes[index])
			started = true
			continue
		}
		if !quoted && (character == ' ' || character == '\t' || character == '\r' || character == '\n') {
			flush()
			continue
		}
		builder.WriteRune(character)
		started = true
	}
	if quoted {
		return nil, errors.New("unterminated quoted text")
	}
	flush()
	return tokens, nil
}

// EverythingSupportsQuery reports whether the current conservative Everything
// translator can preserve all query semantics.
func EverythingSupportsQuery(query Query) bool {
	return len(query.Terms) == 1 && query.Type == EntryTypeAny &&
		len(query.Extensions) == 0 && len(query.Sizes) == 0 &&
		query.ModifiedWithin == 0 && query.Content == ""
}

// BuildEverythingQueryArgs translates a query only when all of its semantics
// can be faithfully represented by the existing Everything invocation.
func BuildEverythingQueryArgs(query Query, limit int) ([]string, error) {
	if !EverythingSupportsQuery(query) {
		return nil, errors.New("query cannot be faithfully expressed by Everything")
	}
	return buildEverythingArgs(query.Terms[0], limit), nil
}
