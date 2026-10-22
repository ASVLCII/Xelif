package internal

import (
	"context"
	"fmt"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/archiveviewer"
)

const (
	archivePreviewLimit  int64 = 1024 * 1024
	viewerPageScrollStep       = 10
)

type archiveOperationMsg struct {
	kind    archiveviewer.ActionKind
	member  string
	content []byte
	err     error
}

func (m *model) openArchive(path string) error {
	if err := m.archiveViewer.Open(path); err != nil {
		return err
	}
	m.setArchiveViewerSize()
	return nil
}

func (m *model) setArchiveViewerSize() {
	m.archiveViewer.SetDimensions(3*m.fullWidth/4, 3*m.fullHeight/4) //nolint:mnd // Archive lists need space.
}

func (m *model) handleArchiveViewerKey(key string) tea.Cmd {
	switch {
	case slices.Contains(common.Hotkeys.Quit, key), slices.Contains(common.Hotkeys.CancelTyping, key):
		_ = m.archiveViewer.Close()
	case slices.Contains(common.Hotkeys.ListUp, key):
		m.archiveViewer.Move(-1)
	case slices.Contains(common.Hotkeys.ListDown, key):
		m.archiveViewer.Move(1)
	case slices.Contains(common.Hotkeys.PageUp, key):
		m.archiveViewer.Move(-viewerPageScrollStep)
	case slices.Contains(common.Hotkeys.PageDown, key):
		m.archiveViewer.Move(viewerPageScrollStep)
	case slices.Contains(common.Hotkeys.ParentDirectory, key):
		return m.handleArchiveAction(m.archiveViewer.Back())
	case slices.Contains(common.Hotkeys.Confirm, key), slices.Contains(common.Hotkeys.ConfirmTyping, key):
		return m.handleArchiveAction(m.archiveViewer.Enter())
	case key == "e":
		return m.handleArchiveAction(m.archiveViewer.ExtractFocused())
	}
	return nil
}

func (m *model) handleArchiveViewerClick(mouse tea.Mouse) tea.Cmd {
	overlayX := m.fullWidth/common.CenterDivisor - m.archiveViewer.GetWidth()/common.CenterDivisor
	overlayY := m.fullHeight/common.CenterDivisor - m.archiveViewer.GetHeight()/common.CenterDivisor
	if mouse.X < overlayX || mouse.X >= overlayX+m.archiveViewer.GetWidth() ||
		mouse.Y < overlayY || mouse.Y >= overlayY+m.archiveViewer.GetHeight() {
		return nil
	}
	action := m.archiveViewer.HandleClick(mouse.X-overlayX, mouse.Y-overlayY, time.Now())
	return m.handleArchiveAction(action)
}

func (m *model) handleArchiveAction(action archiveviewer.Action) tea.Cmd {
	switch action.Kind {
	case archiveviewer.NoAction:
		return nil
	case archiveviewer.CloseAction:
		_ = m.archiveViewer.Close()
		return nil
	case archiveviewer.PreviewAction:
		member := action.Member
		return func() tea.Msg {
			content, err := m.archiveViewer.Read(context.Background(), member, archivePreviewLimit)
			return archiveOperationMsg{kind: action.Kind, member: member, content: content, err: err}
		}
	case archiveviewer.ExtractAction:
		member := action.Member
		destination := m.getFocusedFilePanel().Location
		return func() tea.Msg {
			err := m.archiveViewer.Extract(context.Background(), member, destination)
			return archiveOperationMsg{kind: action.Kind, member: member, err: err}
		}
	default:
		return nil
	}
}

func (m *model) applyArchiveOperation(msg archiveOperationMsg) {
	if msg.err != nil {
		m.showUserError("Archive operation failed", msg.err)
		return
	}
	switch msg.kind {
	case archiveviewer.NoAction, archiveviewer.CloseAction:
		return
	case archiveviewer.PreviewAction:
		content := string(msg.content)
		if !archiveviewer.IsText(msg.content) {
			content = "Binary member preview is not available. Extract it to open with its associated application."
		}
		m.textViewer.Open("Archive: "+msg.member, content)
		m.setTextViewerSize()
	case archiveviewer.ExtractAction:
		m.fileModel.UpdateFilePanelsIfNeeded(true)
		m.textViewer.Open("Archive extraction",
			fmt.Sprintf("Extracted %q without overwriting existing files.", msg.member))
		m.setTextViewerSize()
	}
}
