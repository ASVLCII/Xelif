package filepanel

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yorukot/superfile/src/config/icon"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui"
	"github.com/yorukot/superfile/src/internal/ui/rendering"
	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

/*
- TODO: Write File Panel Specific unit test
  - Individual panel resizes
  - Footer content of filepanel changes due to resizing
  - i Only mode icons remains on smaller
  - ii Other things that change too
  - Other panels like clipboard and metadata's content changes too on resize
*/
func (m *Model) Render(focused bool) string {
	r := ui.FilePanelRenderer(m.height, m.width, focused)

	m.renderTopBar(r)
	m.renderSearchBar(r)
	m.renderFooter(r, m.SelectedCount())
	if m.NeedRenderHeaders() {
		m.renderColumnHeaders(r)
	}
	m.renderFileEntries(r)
	return r.Render()
}

func (m *Model) renderTopBar(r *rendering.Renderer) {
	tabBar := m.renderTabBar()
	prefixWidth := ansi.StringWidth(tabBar) + ansi.StringWidth(common.FilePanelTopDirectoryIcon)
	availableWidth := m.GetContentWidth() - prefixWidth
	if m.TabCount() == 1 {
		availableWidth -= len(" [+]")
	}
	breadcrumbs := ""
	if availableWidth > 0 {
		breadcrumbs, _ = m.headerBreadcrumbs(availableWidth)
	}
	directoryColor := common.GetElementIcon("folder", true, false, common.Config.Nerdfont).Color
	directoryIcon := common.FilePanelTopDirectoryIconStyle.Foreground(lipgloss.Color(directoryColor)).
		Render(ansi.Strip(common.FilePanelTopDirectoryIcon))
	line := tabBar + directoryIcon + common.FilePanelTopPathStyle.Render(breadcrumbs)
	if m.TabCount() == 1 {
		line += strings.Repeat(" ", max(1, m.GetContentWidth()-ansi.StringWidth(line)-len("[+]"))) + "[+]"
	}
	r.AddLines(line)
	searchLabel, commandLabel := m.searchActionLabels()
	r.AddLines(
		common.FilePanelTopPathStyle.Render(
			searchLabel+"  "+commandLabel+"  ",
		) + sortmodel.ControlStyle().
			Render(m.SortActionLabel()),
	)
}

func (m *Model) renderSearchBar(r *rendering.Renderer) {
	r.AddLines(" " + m.SearchBar.View())
}

// TODO : Unit test this
func (m *Model) renderFooter(r *rendering.Renderer, selectedCount uint) {
	r.SetBorderInfoItems(m.footerInfoItems(selectedCount)...)
}

func (m *Model) footerInfoItems(selectedCount uint) []string {
	sortLabel, sortIcon := m.getSortInfo()
	modeLabel, modeIcon := m.getPanelModeInfo(selectedCount)
	cursorStr := m.getCursorString()

	if common.Config.Nerdfont {
		sortLabel = sortIcon + icon.Space + sortLabel
		modeLabel = modeIcon + icon.Space + modeLabel
	} else {
		// TODO : Figure out if we can set icon.Space to " " if nerdfont is false
		// That would simplify code
		sortLabel = sortIcon + " " + sortLabel
	}

	if !common.Config.ShowPanelFooterInfo {
		return nil
	}

	items := []string{sortLabel, modeLabel, cursorStr}
	border := rendering.NewBorderConfig(0, m.width)
	border.SetInfoItems(items...)
	if border.AreInfoItemsTruncated() {
		return []string{sortIcon, modeIcon, cursorStr}
	}
	return items
}

func (m *Model) renderColumnHeaders(r *rendering.Renderer) {
	var builder strings.Builder
	for _, column := range m.columns {
		builder.WriteString(column.RenderHeader())
	}
	r.AddLines(builder.String())
}

func (m *Model) renderFileEntries(r *rendering.Renderer) {
	if m.Empty() {
		r.AddLines(common.FilePanelNoneText)
		return
	}
	end := min(m.renderIndex+m.PanelElementHeight(), m.ElemCount())
	// Separators depend only on column width and selection, not the row's
	// contents. Keep their styled strings local to this frame so theme and
	// background changes never require cache invalidation.
	type delimiterRender struct {
		text  [2]string
		ready [2]bool
	}
	delimiters := make([]delimiterRender, len(m.columns))
	columns := make([]string, len(m.columns))

	for itemIndex := m.renderIndex; itemIndex < end; itemIndex++ {
		if m.Renaming && itemIndex == m.GetCursor() {
			r.AddLines(m.Rename.View())
			continue
		}
		for columnIndex, column := range m.columns {
			var colData string
			if column.isDelimiter {
				selection := 0
				if m.CheckSelected(m.GetElementAtIdx(itemIndex).Location) {
					selection = 1
				}
				cached := &delimiters[columnIndex]
				if !cached.ready[selection] {
					cached.text[selection] = column.Render(itemIndex)
					cached.ready[selection] = true
				}
				colData = cached.text[selection]
			} else {
				colData = column.Render(itemIndex)
			}
			columns[columnIndex] = colData
		}
		// Join sizes the row once, avoiding repeated builder growth and copies
		// when later columns contain ANSI styling.
		r.AddLines(strings.Join(columns, ""))
	}
}

func (m *Model) getSortInfo() (string, string) {
	iconStr := icon.SortAsc
	if m.SortReversed {
		iconStr = icon.SortDesc
	}
	return sortmodel.SortOptionsShortStr[m.SortKind], iconStr
}

func (m *Model) getPanelModeInfo(selectedCount uint) (string, string) {
	switch m.PanelMode {
	case BrowserMode:
		return "Browser", icon.Browser
	case SelectMode:
		return "Select" + icon.Space + fmt.Sprintf("(%d)", selectedCount), icon.Select
	default:
		return "", ""
	}
}

func (m *Model) getCursorString() string {
	cursor := m.GetCursor()
	if !m.Empty() {
		cursor++ // Convert to 1-based
	}
	return fmt.Sprintf("%d/%d", cursor, m.ElemCount())
}

func (m *Model) renderSelectBox(isSelected bool) string {
	if !common.Config.ShowSelectIcons || m.PanelMode != SelectMode {
		return ""
	}
	if !common.Config.Nerdfont {
		if isSelected {
			return "[x] "
		}
		return "[ ] "
	}

	if m.IsFocused {
		if isSelected {
			return common.CheckboxCheckedFocused
		}
		return common.CheckboxEmptyFocused
	}
	if isSelected {
		return common.CheckboxChecked
	}
	return common.CheckboxEmpty
}

// Checks whether a panel needs re-render due to being invalid or due to directory change
func (m *Model) NeedsReRender() bool {
	if !m.EmptyOrInvalid() {
		return filepath.Dir(m.GetFirstElement().Location) != m.Location
	}
	return true
}
