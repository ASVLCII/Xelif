package internal

import tea "charm.land/bubbletea/v2"

func (m *model) unwrapWorkspaceMessage(msg tea.Msg) (tea.Msg, bool) {
	scoped, ok := msg.(workspaceMessage)
	if !ok {
		return msg, true
	}
	if len(m.workspaces) == 0 || scoped.ID != m.workspaces[m.activeWorkspace].State.ID ||
		scoped.Generation != m.workspaceGeneration {
		return nil, false
	}
	return scoped.Message, true
}

// Only input is consumed: operation completions and lifecycle messages must
// continue through Update even while a setup screen or rename form is open.
func (m *model) handleExclusiveSetupInput(msg tea.Msg) (tea.Cmd, bool) {
	if m.firstUse || m.appearanceOpen {
		switch event := msg.(type) {
		case tea.KeyPressMsg:
			return m.handleAppearanceKey(event.String()), true
		case tea.MouseClickMsg:
			if event.Button == tea.MouseLeft {
				return m.handleAppearanceClick(event.Mouse()), true
			}
			return nil, true
		case tea.PasteMsg, tea.MouseMsg:
			return nil, true
		}
	}
	if m.workspaceRenaming {
		switch msg.(type) {
		case tea.KeyPressMsg, tea.PasteMsg:
			return m.workspaceRenameUpdate(msg), true
		}
	}
	return nil, false
}
