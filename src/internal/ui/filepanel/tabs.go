package filepanel

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/yorukot/superfile/src/internal/common"
)

// TabCount returns the number of tabs in this file panel.
func (m *Model) TabCount() int {
	return len(m.tabs)
}

// ActiveTabIndex returns the zero-based active tab index.
func (m *Model) ActiveTabIndex() int {
	return m.activeTab
}

type visibleTab struct {
	index int
	label string
}

const (
	tabControls         = " [+] [-] "
	breadcrumbReserve   = 8
	minimumTabBudget    = 15
	tabBracketWidth     = 2
	maximumTabNameWidth = 12
)

// Share this layout between rendering and hit testing. On overflow retain the
// active tab and controls; wheel navigation reaches every hidden tab.
func (m *Model) visibleTabs() []visibleTab {
	tabs := make([]visibleTab, 0, len(m.tabs))
	width := len(tabControls)
	for index := range m.tabs {
		label := m.tabLabel(index)
		if index > 0 {
			width++
		}
		width += ansi.StringWidth(label)
		tabs = append(tabs, visibleTab{index: index, label: label})
	}
	contentWidth := m.GetContentWidth()
	if contentWidth <= 0 || len(tabs) == 0 {
		return tabs
	}
	budget := contentWidth - min(breadcrumbReserve, max(0, contentWidth-minimumTabBudget))
	if width <= budget {
		return tabs
	}
	active := min(max(m.activeTab, 0), len(tabs)-1)
	tab := tabs[active]
	name := strings.TrimSuffix(strings.TrimPrefix(tab.label, "["), "]")
	tab.label = "[" + ansi.Truncate(name, max(0, budget-len(tabControls)-tabBracketWidth), "") + "]"
	return []visibleTab{tab}
}

// TabAtContentX returns the tab label at a file-panel content X coordinate.
func (m *Model) TabAtContentX(x int) (int, bool) {
	if len(m.tabs) == 1 {
		return 0, false
	}
	start := 0
	for _, tab := range m.visibleTabs() {
		end := start + ansi.StringWidth(tab.label)
		if x >= start && x < end {
			return tab.index, true
		}
		start = end + 1
	}
	return 0, false
}

func (m *Model) tabLabel(index int) string {
	name := filepath.Base(m.tabs[index].location)
	if name == string(filepath.Separator) || name == "." {
		name = m.tabs[index].location
	}
	return "[" + ansi.Truncate(name, maximumTabNameWidth, "...") + "]"
}

func (m *Model) renderTabBar() string {
	if len(m.tabs) <= 1 {
		return ""
	}

	var line strings.Builder
	for position, tab := range m.visibleTabs() {
		if position > 0 {
			line.WriteByte(' ')
		}
		if tab.index == m.activeTab {
			line.WriteString(common.FilePanelCursorStyle.Render(tab.label))
		} else {
			line.WriteString(common.FilePanelTopPathStyle.Render(tab.label))
		}
	}
	line.WriteString(tabControls)
	return line.String()
}

// TabControlAtContentX maps the visible create/close controls on the tab row.
// A single tab cannot be closed, matching CloseTab's keyboard behavior.
func (m *Model) TabControlAtContentX(x int) (bool, bool) {
	if len(m.tabs) == 1 {
		return x >= m.GetContentWidth()-len("[+]") && x < m.GetContentWidth(), false
	}
	if len(m.tabs) == 0 || x < 0 || x >= m.GetContentWidth() {
		return false, false
	}
	start := 0
	for _, tab := range m.visibleTabs() {
		start += ansi.StringWidth(tab.label) + 1
	}
	return x >= start && x < start+3, len(m.tabs) > 1 && x >= start+4 && x < start+7
}

func (m *Model) tabBarWidth() int {
	return ansi.StringWidth(m.renderTabBar())
}

// TabBarWidth is the rendered tab strip's width in terminal cells.
func (m *Model) TabBarWidth() int {
	return m.tabBarWidth()
}

func (m *Model) hasActiveTab() bool {
	return m.activeTab >= 0 && m.activeTab < len(m.tabs)
}

// NewTab opens path in a new active tab in this file panel.
func (m *Model) NewTab(path string) error {
	path, err := m.resolveDirectory(path)
	if err != nil {
		return err
	}

	m.tabs = append(m.tabs, panelTab{location: path})
	m.activeTab = len(m.tabs) - 1
	if path != m.Location {
		m.applyDirectory(path)
	}
	return nil
}

// CloseTab closes the active tab. A file panel always retains one tab.
func (m *Model) CloseTab() error {
	if len(m.tabs) <= 1 {
		return errors.New("cannot close the last tab")
	}

	closing := m.activeTab
	next := closing
	if closing == len(m.tabs)-1 {
		next--
	}
	target := next
	if target == closing {
		target++
	}
	if err := m.activateTab(target); err != nil {
		return err
	}

	m.tabs = append(m.tabs[:closing], m.tabs[closing+1:]...)
	m.activeTab = next
	return nil
}

// NextTab activates the next tab, wrapping at the end.
func (m *Model) NextTab() error {
	if len(m.tabs) <= 1 {
		return nil
	}
	return m.activateTab((m.activeTab + 1) % len(m.tabs))
}

// PreviousTab activates the previous tab, wrapping at the beginning.
func (m *Model) PreviousTab() error {
	if len(m.tabs) <= 1 {
		return nil
	}
	return m.activateTab((m.activeTab - 1 + len(m.tabs)) % len(m.tabs))
}

// ActivateTab activates a tab by zero-based index.
func (m *Model) ActivateTab(index int) error {
	if index < 0 || index >= len(m.tabs) {
		return fmt.Errorf("tab index out of range: %d", index)
	}
	return m.activateTab(index)
}

func (m *Model) activateTab(index int) error {
	if index == m.activeTab {
		return nil
	}
	path, err := m.resolveDirectory(m.tabs[index].location)
	if err != nil {
		return err
	}
	m.activeTab = index
	if path != m.Location {
		m.applyDirectory(path)
	}
	return nil
}

// Back returns the active tab to its previous directory.
func (m *Model) Back() error {
	if !m.hasActiveTab() {
		return nil
	}
	tab := &m.tabs[m.activeTab]
	if len(tab.backHistory) == 0 {
		return nil
	}

	target := tab.backHistory[len(tab.backHistory)-1]
	path, err := m.resolveDirectory(target)
	if err != nil {
		return err
	}
	current := m.Location
	m.applyDirectory(path)
	tab = &m.tabs[m.activeTab]
	tab.backHistory = tab.backHistory[:len(tab.backHistory)-1]
	tab.forwardHistory = append(tab.forwardHistory, current)
	return nil
}

// Forward advances the active tab after a Back operation.
func (m *Model) Forward() error {
	if !m.hasActiveTab() {
		return nil
	}
	tab := &m.tabs[m.activeTab]
	if len(tab.forwardHistory) == 0 {
		return nil
	}

	target := tab.forwardHistory[len(tab.forwardHistory)-1]
	path, err := m.resolveDirectory(target)
	if err != nil {
		return err
	}
	current := m.Location
	m.applyDirectory(path)
	tab = &m.tabs[m.activeTab]
	tab.forwardHistory = tab.forwardHistory[:len(tab.forwardHistory)-1]
	tab.backHistory = append(tab.backHistory, current)
	return nil
}
