package filepanel

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestSingleTabCleanHeaderAndHitTargets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "folder")
	m := Model{width: 100, Location: path, tabs: []panelTab{{location: path}}}
	require.Empty(t, m.renderTabBar())
	line, spans := m.headerBreadcrumbs(80)
	require.Equal(t, path, line)
	for _, span := range spans {
		clicked, ok := breadcrumbPathAt(spans, span.Start)
		require.True(t, ok)
		require.Equal(t, span.Path, clicked)
	}
	create, closeTab := m.TabControlAtContentX(m.GetContentWidth() - 1)
	require.True(t, create)
	require.False(t, closeTab)
	_, ok := m.TabAtContentX(0)
	require.False(t, ok, "path clicks must not be swallowed by a hidden tab")
	line, spans = m.headerBreadcrumbs(12)
	require.LessOrEqual(t, ansi.StringWidth(line), 12)
	require.True(t, strings.HasPrefix(line, "…"))
	for _, span := range spans {
		require.Positive(t, span.Start)
		require.LessOrEqual(t, span.End, 12)
	}
}
