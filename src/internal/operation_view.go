package internal

// operationView limits presentation, not execution. Mutations still complete
// globally and retain Operations history after their originating view closes.
type operationView struct {
	workspaceID string
	generation  uint64
}

func (m *model) operationSubmissionView() operationView {
	view := operationView{generation: m.workspaceGeneration}
	if len(m.workspaces) > 0 {
		view.workspaceID = m.workspaces[m.activeWorkspace].State.ID
	}
	return view
}

func (v operationView) current(m *model) bool {
	if len(m.workspaces) == 0 {
		return v.workspaceID == ""
	}
	return v.workspaceID == m.workspaces[m.activeWorkspace].State.ID && v.generation == m.workspaceGeneration
}
