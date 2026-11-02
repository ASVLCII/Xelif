package globalsearch

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/yorukot/superfile/src/config/icon"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/i18n"
	"github.com/yorukot/superfile/src/internal/ui"
	"github.com/yorukot/superfile/src/internal/ui/rendering"
)

const (
	renderOverhead   = 7
	horizontalMargin = 4
)

// Render draws the search input, bounded result window, and current provider state.
func (m *Model) Render() string {
	renderer := ui.PromptRenderer(m.maxHeight, m.width)
	renderer.SetBorderTitle(icon.Search + icon.Space + i18n.T("search_everywhere"))
	if m.resultTitle != "" {
		renderer.SetBorderTitle(
			icon.Search + icon.Space + common.TruncateText(m.resultTitle, m.width-horizontalMargin, "..."),
		)
	}
	renderer.AddLines(" " + m.textInput.View())
	renderer.AddSection()

	switch {
	case m.savedResults == nil && strings.TrimSpace(m.textInput.Value()) == "":
		renderer.AddLines(" " + i18n.T("type_to_search"))
		if m.visibleResultCount() > 1 {
			renderer.AddLines(" " + i18n.T("search_filters"))
		}
	case m.loading && len(m.results) == 0:
		renderer.AddLines(" " + i18n.T("searching"))
	case m.errText != "":
		renderer.AddLines(" " + common.PromptFailureStyle.Render(m.errText))
	case len(m.results) == 0:
		renderer.AddLines(" " + i18n.T("no_results"))
	default:
		m.renderResults(renderer)
	}

	switch {
	case m.savedResults == nil:
		renderer.AddSection()
		identity, _ := providerDescription(runtime.GOOS, m.GetScope())
		provider := m.provider
		if provider == "" {
			provider = identity.Name
		}
		scope := "indexed: system index"
		hint := "Ctrl+S: folder · Enter: reveal"
		if m.GetScope() == ScopeFolder {
			scope = "folder: " + common.MakePrintableWithEscCheck(m.root, false)
			hint = "Ctrl+S: indexed · Enter: reveal"
		}
		status := common.MakePrintableWithEscCheck(provider, false) + " · " + scope
		renderer.AddLines(" " + common.TruncateText(status, m.width-horizontalMargin, "..."))
		renderer.AddLines(" " + common.TruncateText(hint, m.width-horizontalMargin, "..."))
		if m.notice != "" && m.maxHeight >= 10 {
			renderer.AddLines(
				" " + common.TruncateText(
					common.MakePrintableWithEscCheck(m.notice, false),
					m.width-horizontalMargin,
					"...",
				),
			)
		}
	case m.notice != "":
		renderer.AddSection()
		renderer.AddLines(" " + common.TruncateText(m.notice, m.width-horizontalMargin, "..."))
	case m.partial:
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
		line := " " + common.TruncateTextBeginning(
			common.MakePrintableWithEscCheck(path, false),
			m.width-horizontalMargin,
			"...",
		)
		if index == m.cursor {
			line = common.ModalCursorStyle.Render(line)
		}
		renderer.AddLines(line)
	}
}

func (m *Model) visibleResultCount() int {
	overhead := renderOverhead
	if m.savedResults != nil {
		overhead = 6
	} else if m.notice != "" && m.maxHeight >= 10 {
		overhead++
	}
	return max(1, m.maxHeight-overhead)
}
