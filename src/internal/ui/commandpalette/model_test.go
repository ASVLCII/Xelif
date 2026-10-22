package commandpalette

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func testItems() []Item {
	return []Item{
		{
			ID: "copy", Label: "Copy path", Category: "Files",
			Keywords: []string{"clipboard", "location"}, Hotkeys: []string{"ctrl+c"},
		},
		{
			ID: "branch", Label: "Git branch", Category: "Git",
			Keywords: []string{"checkout", "switch"}, Hotkeys: []string{"g", "b"},
		},
		{
			ID: "delete", Label: "Delete permanently", Category: "Files",
			Keywords: []string{"remove"}, Disabled: true, DisabledReason: "read-only mode",
		},
	}
}

func TestNewCopiesItemsAndStartsClosed(t *testing.T) {
	t.Parallel()

	items := testItems()
	m := New(items)
	items[0].Label = "mutated"
	items[0].Keywords[0] = "mutated"

	assert.False(t, m.IsOpen())
	assert.Equal(t, "Copy path", m.items[0].Label)
	assert.Equal(t, "clipboard", m.items[0].Keywords[0])
	assert.Equal(t, MinWidth, m.GetWidth())
	assert.Equal(t, MinHeight, m.GetMaxHeight())
}

func TestOpenAndCloseResetTransientState(t *testing.T) {
	t.Parallel()

	m := New(testItems())
	assert.Nil(t, m.Open())
	assert.True(t, m.IsOpen())

	_, _ = m.HandleUpdate(utils.TeaRuneKeyMsg("git"))
	_, _ = m.HandleUpdate(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Close()

	assert.False(t, m.IsOpen())
	assert.Empty(t, m.textInput.Value())

	_ = m.Open()
	assert.Len(t, m.filtered, len(testItems()))
	assert.Zero(t, m.cursor)
	assert.Zero(t, m.renderIndex)
}

func TestClosedModelIgnoresUpdates(t *testing.T) {
	t.Parallel()

	m := New(testItems())
	action, cmd := m.HandleUpdate(utils.TeaRuneKeyMsg("c"))

	require.IsType(t, common.NoAction{}, action)
	assert.Nil(t, cmd)
	assert.Empty(t, m.textInput.Value())
}

func TestQueryFiltersAcrossLabelCategoryAndKeywords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{name: "label case insensitive", query: "COPY", want: []string{"copy"}},
		{name: "category", query: "git", want: []string{"branch"}},
		{name: "keyword", query: "checkout", want: []string{"branch"}},
		{name: "fuzzy", query: "dlt prm", want: []string{"delete"}},
		{name: "no match", query: "archive", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := New(testItems())
			_ = m.Open()

			_, _ = m.HandleUpdate(utils.TeaRuneKeyMsg(tt.query))

			assert.Equal(t, tt.want, filteredIDs(m.filtered))
		})
	}
}

func TestFilteringRanksRelevantMatchesAndKeepsTiesStable(t *testing.T) {
	t.Parallel()

	m := New([]Item{
		{ID: "stable-a", Label: "Reopen alpha"},
		{ID: "keyword", Label: "Unrelated", Keywords: []string{"open"}},
		{ID: "exact", Label: "Open"},
		{ID: "prefix", Label: "Open settings"},
		{ID: "stable-b", Label: "Reopen beta"},
	})
	_ = m.Open()

	_, _ = m.HandleUpdate(utils.TeaRuneKeyMsg("open"))

	assert.Equal(t, []string{"exact", "prefix", "keyword", "stable-a", "stable-b"}, filteredIDs(m.filtered))
}

func TestFilteringDoesNotTreatHotkeysAsSearchTerms(t *testing.T) {
	t.Parallel()

	m := New([]Item{{ID: "copy", Label: "Copy", Hotkeys: []string{"ctrl+shift+c"}}})
	_ = m.Open()
	_, _ = m.HandleUpdate(utils.TeaRuneKeyMsg("shift"))

	assert.Empty(t, m.filtered)
}

func TestEmptyQueryRestoresOriginalOrder(t *testing.T) {
	t.Parallel()

	m := New(testItems())
	_ = m.Open()
	m.textInput.SetValue("git")
	m.refilter()
	m.textInput.SetValue("")
	m.refilter()

	assert.Equal(t, []string{"copy", "branch", "delete"}, filteredIDs(m.filtered))
}

func TestExecuteActionString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "ExecuteAction for copy", ExecuteAction{ID: "copy"}.String())
}

func TestEscapeClosesWithoutAction(t *testing.T) {
	t.Parallel()

	m := New(testItems())
	_ = m.Open()
	action, cmd := m.HandleUpdate(tea.KeyPressMsg{Code: tea.KeyEscape})

	require.IsType(t, common.NoAction{}, action)
	assert.Nil(t, cmd)
	assert.False(t, m.IsOpen())
}

func filteredIDs(items []Item) []string {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, len(items))
	for index, item := range items {
		ids[index] = item.ID
	}
	return ids
}
