package globalsearch

import (
	"path/filepath"
	"strings"

	"github.com/yorukot/superfile/src/config/icon"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/i18n"
	"github.com/yorukot/superfile/src/internal/ui"
	"github.com/yorukot/superfile/src/internal/ui/rendering"
)

const (
	renderOverhead   = 6
	horizontalMargin = 4
)

// Render draws the search input, bounded result window, and current provider state.
func (m *Model) Render() string {
	renderer := ui.PromptRenderer(m.maxHeight, m.width)
	renderer.SetBorderTitle(icon.Search + icon.Space + i18n.T("search_everywhere"))
	renderer.AddLines(" " + m.textInput.View())
	renderer.AddSection()

	switch {
	case strings.TrimSpace(m.textInput.Value()) == "":
		renderer.AddLines(" "+i18n.T("type_to_search"), " "+i18n.T("search_filters"))
	case m.loading && len(m.results) == 0:
		renderer.AddLines(" " + i18n.T("searching"))
	case m.errText != "":
		renderer.AddLines(" " + common.PromptFailureStyle.Render(m.errText))
	case len(m.results) == 0:
		renderer.AddLines(" " + i18n.T("no_results"))
	default:
		m.renderResults(renderer)
	}

	if m.notice != "" {
		renderer.AddSection()
		renderer.AddLines(" " + common.TruncateText(m.notice, m.width-horizontalMargin, "..."))
	} else if m.partial {
		renderer.AddSection()
		renderer.AddLines(" Results truncated")
	}
	return renderer.Render()
}

func (m *Model) renderResults(renderer *rendering.Renderer) {
	start := min(m.renderIndex, len(m.results))
	end := min(start+m.visibleResultCount(), len(m.results))
	for index := start; index < end; index++ {
		result := m.results[index]
		name := filepath.Base(result.Path)
		path := result.Path
		if name != "." && name != string(filepath.Separator) {
			path = filepath.Join(filepath.Dir(result.Path), name)
		}
		line := " " + common.TruncateTextBeginning(path, m.width-horizontalMargin, "...")
		if index == m.cursor {
			line = common.ModalCursorStyle.Render(line)
		}
		renderer.AddLines(line)
	}
}

func (m *Model) visibleResultCount() int {
	return max(1, m.maxHeight-renderOverhead)
}
