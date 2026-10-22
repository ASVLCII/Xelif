package globalsearch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseQueryPlainTerms(t *testing.T) {
	t.Parallel()

	query, err := ParseQuery("Quarterly report")

	require.NoError(t, err)
	assert.Equal(t, []string{"Quarterly", "report"}, query.Terms)
	assert.Equal(t, EntryTypeAny, query.Type)
}

func TestParseQueryCombinedFiltersAndQuotedContent(t *testing.T) {
	t.Parallel()

	query, err := ParseQuery(`annual type:file ext:PDF ext:.txt size:>2k size:<3m modified:<7d content:"Net Revenue"`)

	require.NoError(t, err)
	assert.Equal(t, []string{"annual"}, query.Terms)
	assert.Equal(t, EntryTypeFile, query.Type)
	assert.Equal(t, []string{"pdf", "txt"}, query.Extensions)
	assert.Equal(t, []SizePredicate{
		{Comparison: GreaterThan, Bytes: 2 * 1024},
		{Comparison: LessThan, Bytes: 3 * 1024 * 1024},
	}, query.Sizes)
	assert.Equal(t, 7*24*time.Hour, query.ModifiedWithin)
	assert.Equal(t, "Net Revenue", query.Content)
}

func TestParseQuerySupportsEscapesInsideQuotes(t *testing.T) {
	t.Parallel()

	query, err := ParseQuery(`content:"say \"hello\""`)

	require.NoError(t, err)
	assert.Equal(t, `say "hello"`, query.Content)
}

func TestParseQueryRejectsInvalidSyntax(t *testing.T) {
	t.Parallel()

	tests := []string{
		`content:"unterminated`,
		`content:unquoted`,
		`content:""`,
		`type:device`,
		`type:file type:folder`,
		`ext:`,
		`size:2k`,
		`size:>wat`,
		`modified:>7d`,
		`modified:<0d`,
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			_, err := ParseQuery(input)
			assert.Error(t, err)
		})
	}
}

func TestEverythingTranslationOnlyAcceptsFaithfulPlainQuery(t *testing.T) {
	t.Parallel()

	plain, err := ParseQuery("report")
	require.NoError(t, err)
	assert.True(t, EverythingSupportsQuery(plain))
	args, err := BuildEverythingQueryArgs(plain, 25)
	require.NoError(t, err)
	assert.Equal(t, buildEverythingArgs("report", 25), args)

	for _, input := range []string{"two terms", "type:file", "ext:txt", `content:"needle"`} {
		query, parseErr := ParseQuery(input)
		require.NoError(t, parseErr)
		assert.False(t, EverythingSupportsQuery(query), input)
		_, buildErr := BuildEverythingQueryArgs(query, 25)
		assert.Error(t, buildErr, input)
	}
}
