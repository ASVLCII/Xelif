package internal

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/session"
)

func (m *model) snapshotSession() session.State {
	panels, focusedPanel, showDotFiles := m.fileModel.SnapshotSession()
	return session.State{
		Version:        session.CurrentVersion,
		FocusedPanel:   focusedPanel,
		ShowDotFiles:   showDotFiles,
		PreviewVisible: m.fileModel.FilePreview.IsOpen(),
		FooterVisible:  m.toggleFooter,
		Panels:         panels,
	}
}

func (m *model) saveLastSession() error {
	if m.sessionStore == nil {
		return nil
	}
	m.ensureWorkspaces()
	m.captureWorkspace()
	state := session.AppState{
		Version:           session.CurrentAppVersion,
		ActiveWorkspaceID: m.workspaces[m.activeWorkspace].State.ID,
		Workspaces:        make([]session.WorkspaceState, len(m.workspaces)),
	}
	for i, w := range m.workspaces {
		state.Workspaces[i] = w.State
	}
	if err := m.sessionStore.SaveAppLast(state); err != nil {
		return fmt.Errorf("save last session: %w", err)
	}
	return nil
}

func (m *model) restoreLastSession() error {
	if m.sessionStore == nil {
		return nil
	}
	state, err := m.sessionStore.LoadAppLast()
	if errors.Is(err, session.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("restore last session: %w", err)
	}
	m.ensureWorkspaces()
	m.cancelWorkspaceWork()
	m.workspaces = make([]workspaceRuntime, len(state.Workspaces))
	m.activeWorkspace = 0
	for i, w := range state.Workspaces {
		m.workspaces[i] = workspaceRuntime{State: w}
		if w.ID == state.ActiveWorkspaceID {
			m.activeWorkspace = i
		}
	}
	_ = m.restoreActiveWorkspace()
	return nil
}

func (m *model) saveWorkspace(name string) error {
	if m.sessionStore == nil {
		return errors.New("workspace storage is unavailable")
	}
	if err := m.sessionStore.SaveWorkspace(name, m.snapshotSession()); err != nil {
		return fmt.Errorf("save workspace %q: %w", name, err)
	}
	return nil
}

func (m *model) loadWorkspace(name string) (tea.Cmd, error) {
	if !m.canAddWorkspace() {
		return nil, errors.New("close an open workspace before loading another")
	}
	if m.sessionStore == nil {
		return nil, errors.New("workspace storage is unavailable")
	}
	m.ensureWorkspaces()
	state, err := m.sessionStore.LoadWorkspace(name)
	if err != nil {
		return nil, fmt.Errorf("load workspace %q: %w", name, err)
	}
	if !m.workspaceSwitchAllowed() {
		return nil, errors.New("finish or cancel the current dialog before loading a workspace")
	}
	id, err := session.NewWorkspaceID()
	if err != nil {
		return nil, err
	}
	m.captureWorkspace()
	m.cancelWorkspaceWork()
	m.workspaces = append(
		m.workspaces,
		workspaceRuntime{State: session.WorkspaceState{ID: id, Title: m.uniqueWorkspaceTitle(name, -1), Layout: state}},
	)
	m.activeWorkspace = len(m.workspaces) - 1
	return m.restoreActiveWorkspace(), nil
}

func (m *model) deleteWorkspace(name string) error {
	if m.sessionStore == nil {
		return errors.New("workspace storage is unavailable")
	}
	if err := m.sessionStore.DeleteWorkspace(name); err != nil {
		return fmt.Errorf("delete workspace %q: %w", name, err)
	}
	return nil
}
