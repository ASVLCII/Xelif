package internal

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/ui/metadata"
	"github.com/yorukot/superfile/src/internal/ui/notify"
	"github.com/yorukot/superfile/src/internal/ui/processbar"
	"github.com/yorukot/superfile/src/internal/ui/spferror"
)

type ModelUpdateMessage interface {
	ApplyToModel(m *model) tea.Cmd
	GetReqID() int
}

type BaseMessage struct {
	reqID int
}

func (msg BaseMessage) GetReqID() int {
	return msg.reqID
}

type UndoOperationMsg struct {
	BaseMessage

	journal   *undoAction
	remaining *undoAction
	err       error
}

func (msg UndoOperationMsg) ApplyToModel(m *model) tea.Cmd {
	if m.lastUndo == msg.journal {
		m.lastUndo = msg.remaining
	}
	if msg.journal != nil {
		completedFrom := 0
		if msg.remaining != nil {
			completedFrom = len(msg.remaining.moves)
		}
		reversed := make([]pathMove, 0, len(msg.journal.moves)-completedFrom)
		for _, move := range msg.journal.moves[completedFrom:] {
			reversed = append(reversed, pathMove{from: move.to, to: move.from})
		}
		m.updateCatalogMoves(reversed)
	}
	m.showUserError("Could not undo", msg.err)
	return nil
}

type PasteOperationMsg struct {
	BaseMessage

	state               processbar.ProcessState
	moves               []pathMove
	cut                 bool
	clipboardGeneration uint64
}

func NewPasteOperationMsg(state processbar.ProcessState, reqID int) PasteOperationMsg {
	return PasteOperationMsg{
		state: state,
		BaseMessage: BaseMessage{
			reqID: reqID,
		},
	}
}

func newPasteOperationMsgWithContext(
	state processbar.ProcessState,
	reqID int,
	cut bool,
	clipboardGeneration uint64,
	moves []pathMove,
) PasteOperationMsg {
	msg := NewPasteOperationMsg(state, reqID)
	msg.moves = append([]pathMove(nil), moves...)
	msg.cut = cut
	msg.clipboardGeneration = clipboardGeneration
	return msg
}

func (msg PasteOperationMsg) ApplyToModel(m *model) tea.Cmd {
	terminal := msg.state == processbar.Failed || msg.state == processbar.Successful ||
		msg.state == processbar.Cancelled
	if terminal && msg.cut && m.clipboard.Generation() == msg.clipboardGeneration {
		m.clipboard.Reset(false)
	}
	if terminal && msg.cut && len(msg.moves) > 0 {
		m.recordUndo("move", msg.moves)
		m.updateCatalogMoves(msg.moves)
	}
	return nil
}

type CreateOperationMsg struct {
	BaseMessage

	state processbar.ProcessState
}

func NewCreateOperationMsg(state processbar.ProcessState, reqID int) CreateOperationMsg {
	return CreateOperationMsg{
		state: state,
		BaseMessage: BaseMessage{
			reqID: reqID,
		},
	}
}

func (msg CreateOperationMsg) ApplyToModel(m *model) tea.Cmd {
	return nil
}

type DeleteOperationMsg struct {
	BaseMessage

	state processbar.ProcessState
}

func NewDeleteOperationMsg(state processbar.ProcessState, reqID int) DeleteOperationMsg {
	return DeleteOperationMsg{
		state: state,
		BaseMessage: BaseMessage{
			reqID: reqID,
		},
	}
}

func (msg DeleteOperationMsg) ApplyToModel(m *model) tea.Cmd {
	// Remove selection
	m.getFocusedFilePanel().ResetSelected()
	return nil
}

type ProcessBarUpdateMsg struct {
	BaseMessage

	pMsg processbar.UpdateMsg
}

func (msg ProcessBarUpdateMsg) ApplyToModel(m *model) tea.Cmd {
	cmd, err := msg.pMsg.Apply(&m.processBarModel)
	if err != nil {
		slog.Error("Error applying processbar update", "error", err)
	}
	return processCmdToTeaCmd(cmd)
}

type CompressOperationMsg struct {
	BaseMessage

	state processbar.ProcessState
}

func NewCompressOperationMsg(state processbar.ProcessState, reqID int) CompressOperationMsg {
	return CompressOperationMsg{
		state: state,
		BaseMessage: BaseMessage{
			reqID: reqID,
		},
	}
}

func (msg CompressOperationMsg) ApplyToModel(_ *model) tea.Cmd {
	return nil
}

type ExtractOperationMsg struct {
	BaseMessage

	state processbar.ProcessState
}

func NewExtractOperationMsg(state processbar.ProcessState, reqID int) ExtractOperationMsg {
	return ExtractOperationMsg{
		state: state,
		BaseMessage: BaseMessage{
			reqID: reqID,
		},
	}
}

func (msg ExtractOperationMsg) ApplyToModel(_ *model) tea.Cmd {
	return nil
}

type MetadataMsg struct {
	BaseMessage

	err error

	meta            metadata.Metadata
	metadataFocused bool
}

func NewMetadataMsg(meta metadata.Metadata, metadataFocused bool, reqID int) MetadataMsg {
	return MetadataMsg{
		meta:            meta,
		metadataFocused: metadataFocused,
		BaseMessage: BaseMessage{
			reqID: reqID,
		},
	}
}

func (msg MetadataMsg) ApplyToModel(m *model) tea.Cmd {
	if m.metadataReqID != 0 && msg.reqID != m.metadataReqID {
		return nil
	}
	if msg.err != nil {
		m.fileMetaData.SetInfoMsg("Metadata cancelled or unavailable")
		return nil
	}
	m.fileMetaData.SetMetadataCache(msg.meta, msg.metadataFocused)
	selectedItem := m.getFocusedFilePanel().GetFocusedItemPtr()
	if selectedItem == nil {
		slog.Debug("Panel empty or cursor invalid. Ignoring MetadataMsg")
		return nil
	}
	if selectedItem.Location != msg.meta.GetPath() {
		slog.Debug("MetadataMsg for older files. Ignoring",
			"currentItem", selectedItem.Location, "msgItem", msg.meta.GetPath())
		return nil
	}
	if (m.focusPanel == metadataFocus) != msg.metadataFocused {
		slog.Debug("MetadataMsg for older state. Ignoring",
			"actualFocus", m.focusPanel, "msgFocus", msg.metadataFocused)
		return nil
	}
	m.fileMetaData.SetMetadata(msg.meta, msg.metadataFocused)
	return nil
}

type NotifyModalUpdateMsg struct {
	BaseMessage

	m notify.Model
}

func NewNotifyModalMsg(m notify.Model, reqID int) NotifyModalUpdateMsg {
	return NotifyModalUpdateMsg{
		m: m,
		BaseMessage: BaseMessage{
			reqID: reqID,
		},
	}
}

func (msg NotifyModalUpdateMsg) ApplyToModel(m *model) tea.Cmd {
	m.notifyModel = msg.m
	return nil
}

type SpfErrorModalUpdateMsg struct {
	BaseMessage

	m spferror.Model
}

func NewSpfErrorModalMsg(m spferror.Model, reqID int) SpfErrorModalUpdateMsg {
	return SpfErrorModalUpdateMsg{
		m: m,
		BaseMessage: BaseMessage{
			reqID: reqID,
		},
	}
}

func (msg SpfErrorModalUpdateMsg) ApplyToModel(m *model) tea.Cmd {
	m.spfError = msg.m
	return nil
}
