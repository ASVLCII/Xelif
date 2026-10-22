package commandpalette

import (
	"strings"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui"
	"github.com/yorukot/superfile/src/internal/ui/rendering"
)

const renderOverhead = 6

// Render draws the filter input and the bounded command window.
func (m *Model) Render() string {
	renderer := ui.PromptRenderer(m.maxHeight, m.width)
	renderer.SetBorderTitle("Command Palette")
	renderer.AddLines(" " + m.textInput.View())
	renderer.AddSection()

	if len(m.filtered) == 0 {
		renderer.AddLines(" No matching commands")
		return renderer.Render()
	}
	m.renderItems(renderer)
	return renderer.Render()
}

func (m *Model) renderItems(renderer *rendering.Renderer) {
	start := min(m.renderIndex, len(m.filtered))
	end := min(start+m.visibleItemCount(), len(m.filtered))
	for index := start; index < end; index++ {
		item := m.filtered[index]
		parts := []string{" " + item.Label}
		if item.Category != "" {
			parts = append(parts, "["+item.Category+"]")
		}
		if hotkeys := effectiveHotkeys(item.Hotkeys); len(hotkeys) > 0 {
			parts = append(parts, strings.Join(hotkeys, ", "))
		}
		if item.Disabled {
			reason := strings.TrimSpace(item.DisabledReason)
			if reason == "" {
				reason = "unavailable"
			}
			parts = append(parts, "(disabled: "+reason+")")
		}
		line := strings.Join(parts, "  ")
		if index == m.cursor {
			line = common.ModalCursorStyle.Render(line)
		}
		renderer.AddLines(line)
	}
}

func effectiveHotkeys(hotkeys []string) []string {
	seen := make(map[string]struct{}, len(hotkeys))
	effective := make([]string, 0, len(hotkeys))
	for _, hotkey := range hotkeys {
		hotkey = strings.TrimSpace(hotkey)
		if hotkey == "" {
			continue
		}
		if _, exists := seen[hotkey]; exists {
			continue
		}
		seen[hotkey] = struct{}{}
		effective = append(effective, hotkey)
	}
	return effective
}

func (m *Model) visibleItemCount() int {
	return max(1, m.maxHeight-renderOverhead)
}
