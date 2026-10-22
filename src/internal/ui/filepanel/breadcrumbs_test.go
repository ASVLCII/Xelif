package filepanel

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/rendering"
)

func TestRenderBreadcrumbsBuildsClickableAncestorSpans(t *testing.T) {
	root := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	location := filepath.Join(root, "alpha", "beta")

	line, spans := renderBreadcrumbs(location, 100)

	separator := " > "
	expected := root + separator + "alpha" + separator + "beta"
	require.Equal(t, expected, line)
	require.Len(t, spans, 3)
	assert.Equal(t, BreadcrumbSpan{Start: 0, End: ansi.StringWidth(root), Path: root}, spans[0])
	assert.Equal(t, filepath.Join(root, "alpha"), spans[1].Path)
	assert.Equal(t, location, spans[2].Path)
	assert.Equal(t, "alpha", lineAtSpan(line, spans[1]))
	assert.Equal(t, "beta", lineAtSpan(line, spans[2]))
}

func TestRenderBreadcrumbsKeepsRightmostWholeSegmentsWhenNarrow(t *testing.T) {
	root := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	location := filepath.Join(root, "alpha", "beta", "gamma")
	expected := "… > beta > gamma"

	line, spans := renderBreadcrumbs(location, ansi.StringWidth(expected))

	require.Equal(t, expected, line)
	require.Len(t, spans, 2)
	assert.Equal(t, ansi.StringWidth("… > "), spans[0].Start)
	assert.Equal(t, filepath.Join(root, "alpha", "beta"), spans[0].Path)
	assert.Equal(t, location, spans[1].Path)
}

func TestRenderBreadcrumbsTruncatesAnOversizedLeaf(t *testing.T) {
	root := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	location := filepath.Join(root, "alpha", "verylonggamma")

	line, spans := renderBreadcrumbs(location, 12)

	require.Equal(t, "… > ...gamma", line)
	require.Len(t, spans, 1)
	assert.Equal(t, ansi.StringWidth("… > "), spans[0].Start)
	assert.Equal(t, location, spans[0].Path)
	assert.Equal(t, "...gamma", lineAtSpan(line, spans[0]))
}

func TestBreadcrumbPathAtHitsLabelsButNotSeparators(t *testing.T) {
	root := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	location := filepath.Join(root, "alpha", "beta")
	_, spans := renderBreadcrumbs(location, 100)

	path, ok := breadcrumbPathAt(spans, spans[1].Start)
	require.True(t, ok)
	assert.Equal(t, filepath.Join(root, "alpha"), path)

	_, ok = breadcrumbPathAt(spans, spans[0].End)
	assert.False(t, ok)
}

func TestBreadcrumbPathAtContentXAccountsForTopBarIcon(t *testing.T) {
	root := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	panel := Model{Location: filepath.Join(root, "alpha"), width: 80}
	originalIcon := common.FilePanelTopDirectoryIcon
	common.FilePanelTopDirectoryIcon = " D " //nolint:reassign // Test temporarily overrides a UI global.
	t.Cleanup(func() {
		common.FilePanelTopDirectoryIcon = originalIcon //nolint:reassign // Restore the UI global.
	})

	path, ok := panel.BreadcrumbPathAtContentX(ansi.StringWidth(common.FilePanelTopDirectoryIcon))

	require.True(t, ok)
	assert.Equal(t, root, path)
	_, ok = panel.BreadcrumbPathAtContentX(0)
	assert.False(t, ok)
}

func TestRenderTopBarUsesBreadcrumbPresentation(t *testing.T) {
	root := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	panel := Model{Location: filepath.Join(root, "alpha"), width: 80}
	originalIcon := common.FilePanelTopDirectoryIcon
	common.FilePanelTopDirectoryIcon = " D " //nolint:reassign // Test temporarily overrides a UI global.
	t.Cleanup(func() {
		common.FilePanelTopDirectoryIcon = originalIcon //nolint:reassign // Restore the UI global.
	})
	r := rendering.NewRendererWithAutoFixConfig(rendering.DefaultRendererConfig(3, 80))

	panel.renderTopBar(r)
	firstLine := strings.TrimRight(strings.Split(r.Render(), "\n")[0], " ")

	assert.Equal(t, " D "+root+" > alpha", ansi.Strip(firstLine))
}

func lineAtSpan(line string, span BreadcrumbSpan) string {
	return ansi.Cut(line, span.Start, span.End)
}
