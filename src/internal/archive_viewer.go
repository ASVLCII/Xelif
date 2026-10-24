package internal

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/archivebrowser"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/archiveviewer"
)

const (
	archivePreviewLimit  int64 = 1024 * 1024
	viewerPageScrollStep       = 10
)

type archiveOperationMsg struct {
	operation archiveviewer.Operation
	kind      archiveviewer.ActionKind
	member    string
	content   []byte
	err       error
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
	case archiveviewer.PreviewAction, archiveviewer.ExtractAction:
		operation, err := m.archiveViewer.StartOperation()
		if err != nil {
			m.showUserError("Archive operation failed", err)
			return nil
		}
		destination := m.getFocusedFilePanel().Location
		return m.trackTask(
			"Archive member",
			operation.ArchivePath+" : "+action.Member,
			destination,
			false,
			operation.Context,
			operation.Cancel,
			archiveOperationCmd(operation, action, destination),
		)
	default:
		return nil
	}
}

func archiveOperationCmd(operation archiveviewer.Operation, action archiveviewer.Action, destination string) tea.Cmd {
	return func() tea.Msg {
		msg := archiveOperationMsg{operation: operation, kind: action.Kind, member: action.Member}
		if err := operation.Context.Err(); err != nil {
			msg.err = err
			return msg
		}
		archive, err := archivebrowser.Open(operation.ArchivePath)
		if err != nil {
			msg.err = err
			return msg
		}
		defer archive.Close()

		switch action.Kind {
		case archiveviewer.PreviewAction:
			msg.content, msg.err = archive.Read(operation.Context, action.Member, archivePreviewLimit)
		case archiveviewer.ExtractAction:
			msg.err = archive.Extract(operation.Context, []string{action.Member}, destination)
		case archiveviewer.NoAction, archiveviewer.CloseAction:
			return msg
		}
		return msg
	}
}

func (m *model) applyArchiveOperation(msg archiveOperationMsg) {
	if !m.archiveViewer.CompleteOperation(msg.operation) || errors.Is(msg.err, context.Canceled) {
		return
	}
	if msg.err != nil {
		m.showTextViewerError("Archive operation failed", msg.err, "")
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
