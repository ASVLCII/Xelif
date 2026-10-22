package sidebar

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFocusDirectoryAtYWithVisibleSearchRow(t *testing.T) {
	sidebar := testModel(1, 0, 12, nil, []directory{
		{Name: "first", Location: "/first"},
		{Name: "second", Location: "/second"},
	})

	assert.True(t, sidebar.IsSearchRowY(3, true))
	path, ok := sidebar.FocusDirectoryAtY(4, true)

	require.True(t, ok)
	assert.Equal(t, "/first", path)
	assert.Equal(t, 0, sidebar.cursor)
}

func TestFocusDirectoryAtYTracksSearchVisibility(t *testing.T) {
	directories := []directory{
		{Name: "first", Location: "/first"},
		{Name: "second", Location: "/second"},
	}

	t.Run("search stays visible without focus", func(t *testing.T) {
		sidebar := testModel(1, 0, 12, nil, directories)

		assert.True(t, sidebar.IsSearchRowY(3, false))
		path, ok := sidebar.FocusDirectoryAtY(4, false)

		require.True(t, ok)
		assert.Equal(t, "/first", path)
		assert.Equal(t, 0, sidebar.cursor)
	})

	t.Run("non-empty search remains visible without sidebar focus", func(t *testing.T) {
		sidebar := testModel(1, 0, 12, nil, directories)
		sidebar.searchBar.SetValue("first")

		assert.True(t, sidebar.IsSearchRowY(3, false))
		path, ok := sidebar.FocusDirectoryAtY(4, false)

		require.True(t, ok)
		assert.Equal(t, "/first", path)
		assert.Equal(t, 0, sidebar.cursor)
	})
}

func TestFocusDirectoryAtYUsesScrolledRenderIndex(t *testing.T) {
	sidebar := testModel(0, 2, 12, nil, []directory{
		{Name: "first", Location: "/first"},
		{Name: "second", Location: "/second"},
		{Name: "third", Location: "/third"},
		{Name: "fourth", Location: "/fourth"},
	})

	path, ok := sidebar.FocusDirectoryAtY(4, true)

	require.True(t, ok)
	assert.Equal(t, "/third", path)
	assert.Equal(t, 2, sidebar.cursor)
}

func TestFocusDirectoryAtYSkipsEveryDividerRow(t *testing.T) {
	directories := []directory{
		{Name: "first", Location: "/first"},
		pinnedDividerDir,
		{Name: "second", Location: "/second"},
	}

	for _, test := range []struct {
		name string
		y    int
	}{
		{name: "leading whitespace", y: 5},
		{name: "divider title", y: 6},
		{name: "trailing whitespace", y: 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			sidebar := testModel(0, 0, 12, nil, directories)

			path, ok := sidebar.FocusDirectoryAtY(test.y, true)

			assert.False(t, ok)
			assert.Empty(t, path)
			assert.Equal(t, 0, sidebar.cursor)
		})
	}

	sidebar := testModel(0, 0, 12, nil, directories)
	path, ok := sidebar.FocusDirectoryAtY(8, true)
	require.True(t, ok)
	assert.Equal(t, "/second", path)
	assert.Equal(t, 2, sidebar.cursor)
}

func TestFocusDirectoryAtYRejectsBoundsAndRowsOutsideRenderedViewport(t *testing.T) {
	directories := []directory{
		{Name: "first", Location: "/first"},
		{Name: "second", Location: "/second"},
		{Name: "third", Location: "/third"},
	}

	for _, test := range []struct {
		name           string
		y              int
		sidebarFocused bool
	}{
		{name: "negative row", y: -1, sidebarFocused: true},
		{name: "top border", y: 0, sidebarFocused: true},
		{name: "title", y: 1, sidebarFocused: true},
		{name: "title spacer", y: 2, sidebarFocused: true},
		{name: "search row", y: 3, sidebarFocused: true},
		{name: "visible search bottom border", y: 6, sidebarFocused: true},
		{name: "unfocused search row", y: 3, sidebarFocused: false},
		{name: "past sidebar", y: 7, sidebarFocused: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			sidebar := testModel(1, 0, 7, nil, directories)

			path, ok := sidebar.FocusDirectoryAtY(test.y, test.sidebarFocused)

			assert.False(t, ok)
			assert.Empty(t, path)
			assert.Equal(t, 1, sidebar.cursor)
		})
	}
}
