package commandpalette

import (
	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/common"
)

// New creates a closed command palette containing an independent copy of items.
func New(items []Item) Model {
	m := Model{
		items:     cloneItems(items),
		textInput: common.GeneratePromptTextInput(),
	}
	m.filtered = cloneItems(m.items)
	m.SetWidth(MinWidth)
	m.SetMaxHeight(MinHeight)
	return m
}

// Open resets and focuses the palette.
func (m *Model) Open() tea.Cmd {
	m.open = true
	m.textInput.SetValue("")
	_ = m.textInput.Focus()
	m.filtered = cloneItems(m.items)
	m.cursor = 0
	m.renderIndex = 0
	return nil
}

// Close closes the palette and clears its query.
func (m *Model) Close() {
	m.open = false
	m.textInput.Blur()
	m.textInput.SetValue("")
	m.filtered = cloneItems(m.items)
	m.cursor = 0
	m.renderIndex = 0
}

func (m *Model) IsOpen() bool {
	return m.open
}

// HandleUpdate processes keyboard and text-input messages while the palette is open.
func (m *Model) HandleUpdate(msg tea.Msg) (common.ModelAction, tea.Cmd) {
	noAction := common.ModelAction(common.NoAction{})
	if !m.open {
		return noAction, nil
	}

	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.Code {
		case tea.KeyEscape:
			m.Close()
			return noAction, nil
		case tea.KeyEnter:
			return m.executeSelected(), nil
		case tea.KeyUp:
			m.navigateUp()
			return noAction, nil
		case tea.KeyDown:
			m.navigateDown()
			return noAction, nil
		case tea.KeyPgUp:
			m.Scroll(-m.visibleItemCount())
			return noAction, nil
		case tea.KeyPgDown:
			m.Scroll(m.visibleItemCount())
			return noAction, nil
		}
	}

	oldQuery := m.textInput.Value()
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	if m.textInput.Value() != oldQuery {
		m.refilter()
	}
	return noAction, cmd
}

func (m *Model) executeSelected() common.ModelAction {
	if m.cursor < 0 || m.cursor >= len(m.filtered) || m.filtered[m.cursor].Disabled {
		return common.NoAction{}
	}
	selected := m.filtered[m.cursor]
	m.Close()
	return ExecuteAction{ID: selected.ID}
}

func cloneItems(items []Item) []Item {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]Item, len(items))
	for index, item := range items {
		cloned[index] = item
		cloned[index].Keywords = append([]string(nil), item.Keywords...)
		cloned[index].Hotkeys = append([]string(nil), item.Hotkeys...)
	}
	return cloned
}
