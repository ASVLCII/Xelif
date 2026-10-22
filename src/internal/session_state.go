package internal

import (
	"errors"
	"fmt"

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
	if err := m.sessionStore.SaveLast(m.snapshotSession()); err != nil {
		return fmt.Errorf("save last session: %w", err)
	}
	return nil
}

func (m *model) restoreLastSession() error {
	if m.sessionStore == nil {
		return nil
	}
	state, err := m.sessionStore.LoadLast()
	if errors.Is(err, session.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("restore last session: %w", err)
	}
	m.applySessionState(state)
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

func (m *model) loadWorkspace(name string) error {
	if m.sessionStore == nil {
		return errors.New("workspace storage is unavailable")
	}
	state, err := m.sessionStore.LoadWorkspace(name)
	if err != nil {
		return fmt.Errorf("load workspace %q: %w", name, err)
	}
	m.applySessionState(state)
	return nil
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

func (m *model) applySessionState(state session.State) {
	fallbackLocation := m.getFocusedFilePanel().Location
	m.fileModel.RestoreSession(state.Panels, state.FocusedPanel, state.ShowDotFiles, fallbackLocation)
	m.fileModel.FilePreview.SetOpen(state.PreviewVisible)
	m.toggleFooter = state.FooterVisible
	m.focusPanel = nonePanelFocus
	m.setHeightValues()
	if m.fullWidth > 0 && m.fullHeight > 0 {
		_ = m.updateComponentDimensions()
	}
}
