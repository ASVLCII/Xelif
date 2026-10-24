package internal

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui"
	"github.com/yorukot/superfile/src/internal/ui/processbar"
)

const (
	operationHeaderRows   = 5
	operationMaxWidth     = 100
	operationMaxHeight    = 28
	operationMinWidth     = 16
	operationMinHeight    = 8
	operationActionsRow   = 2
	operationCancelLabel  = "[Cancel]"
	operationClearLabel   = "[Clear finished]"
	operationDetailsLabel = "[Details]"
	operationCloseLabel   = "[Close]"
	operationTabsBoundary = len("[Operations]  ") + 1
	operationCancelEnd    = len(operationCancelLabel)
	operationClearEnd     = operationCancelEnd + 1 + len(operationClearLabel)
	operationDetailsEnd   = operationClearEnd + 1 + len(operationDetailsLabel)
	operationCloseEnd     = operationDetailsEnd + 1 + len(operationCloseLabel)
)

type operationCenter struct {
	open, background, details    bool
	filter                       textinput.Model
	selected                     string
	cursor, offset, detailOffset int
	rows                         []processbar.Process
	revision                     uint64
	lastFilter                   string
	lastBackground               bool
}

func (m *model) openOperations() {
	m.sidebarModel.SearchBarBlur()
	for i := range m.fileModel.FilePanels {
		m.fileModel.FilePanels[i].SearchBar.Blur()
	}
	m.operations = operationCenter{open: true, filter: common.GeneratePromptTextInput()}
	m.operations.filter.Placeholder = "Filter tasks..."
	_ = m.operations.filter.Focus()
	m.operationRows()
}

func (m *model) operationSize() (int, int) {
	padding := common.CenterDivisor * common.BorderPadding
	return max(
			min(operationMaxWidth, m.fullWidth-padding),
			operationMinWidth,
		), max(
			min(operationMaxHeight, m.fullHeight-padding),
			operationMinHeight,
		)
}

func (m *model) operationRows() []processbar.Process {
	query := strings.ToLower(m.operations.filter.Value())
	revision := m.processBarModel.TaskRevision()
	visible := m.operations.rows
	if visible == nil || revision != m.operations.revision || query != m.operations.lastFilter ||
		m.operations.background != m.operations.lastBackground {
		rows := m.processBarModel.TaskSnapshot()
		visible = rows[:0]
		for _, p := range rows {
			if p.Background != m.operations.background {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(p.TaskName()+" "+p.Source+" "+
				p.Destination+" "+p.CurrentFile+" "+p.ErrorMsg+" "+p.StatusText()), query) {
				continue
			}
			visible = append(visible, p)
		}
		m.operations.rows = visible
		m.operations.revision, m.operations.lastFilter, m.operations.lastBackground = revision, query, m.operations.background
	}
	for i, p := range visible {
		if p.ID == m.operations.selected {
			m.operations.cursor = i
			break
		}
	}
	if len(visible) == 0 {
		m.operations.cursor, m.operations.offset = 0, 0
		m.operations.selected = ""
		return visible
	}
	m.operations.cursor = min(max(m.operations.cursor, 0), len(visible)-1)
	m.operations.selected = visible[m.operations.cursor].ID
	_, height := m.operationSize()
	count := max(height-operationHeaderRows-common.BorderPadding, 1)
	m.operations.offset = min(max(m.operations.offset, m.operations.cursor-count+1), m.operations.cursor)
	return visible
}

func (m *model) moveOperation(delta int) {
	if m.operations.details {
		m.operations.detailOffset = max(m.operations.detailOffset+delta, 0)
		return
	}
	rows := m.operationRows()
	if len(rows) == 0 {
		return
	}
	m.operations.cursor = min(max(m.operations.cursor+delta, 0), len(rows)-1)
	m.operations.selected = rows[m.operations.cursor].ID
	m.operationRows()
}

func (m *model) handleOperationKey(msg tea.KeyPressMsg) tea.Cmd {
	m.operationRows()
	switch msg.String() {
	case "esc":
		if m.operations.details {
			m.operations.details = false
			return nil
		}
		m.operations.open = false
	case "tab", "shift+tab":
		m.operations.background = !m.operations.background
		m.operations.selected = ""
		m.operations.cursor, m.operations.offset = 0, 0
		m.operations.details = false
	case "up":
		m.moveOperation(-1)
	case "down":
		m.moveOperation(1)
	case "pgup":
		_, height := m.operationSize()
		m.moveOperation(-height + operationHeaderRows + common.BorderPadding)
	case "pgdown":
		_, height := m.operationSize()
		m.moveOperation(height - operationHeaderRows - common.BorderPadding)
	case "enter":
		m.operations.details = !m.operations.details
		m.operations.detailOffset = 0
	case "ctrl+x":
		m.processBarModel.Cancel(m.operations.selected)
	case "ctrl+l":
		m.processBarModel.ClearFinished()
		m.operationRows()
	default:
		if !m.operations.details {
			var cmd tea.Cmd
			m.operations.filter, cmd = m.operations.filter.Update(msg)
			m.operations.selected = ""
			m.operations.cursor, m.operations.offset = 0, 0
			return cmd
		}
	}
	return nil
}

func (m *model) handleOperationMouse(msg tea.MouseMsg) {
	m.operationRows()
	if wheel, ok := msg.(tea.MouseWheelMsg); ok {
		if wheel.Button == tea.MouseWheelUp {
			m.moveOperation(-1)
		}
		if wheel.Button == tea.MouseWheelDown {
			m.moveOperation(1)
		}
		return
	}
	click, ok := msg.(tea.MouseClickMsg)
	if !ok || click.Button != tea.MouseLeft {
		return
	}
	w, h := m.operationSize()
	x, y := click.X-(m.fullWidth/2-w/2), click.Y-(m.fullHeight/2-h/2)
	if x <= 0 || x >= w-1 || y <= 0 || y >= h-1 {
		return
	}
	switch y {
	case 1:
		m.operations.background = x >= operationTabsBoundary
		m.operations.selected = ""
		m.operations.cursor, m.operations.offset = 0, 0
		m.operations.details = false
	case operationActionsRow:
		switch {
		case x <= operationCancelEnd:
			m.processBarModel.Cancel(m.operations.selected)
		case x <= operationClearEnd:
			m.processBarModel.ClearFinished()
		case x <= operationDetailsEnd:
			m.operations.details = !m.operations.details
			m.operations.detailOffset = 0
		case x <= operationCloseEnd:
			m.operations.open = false
		}
	default:
		if m.operations.details || y < operationHeaderRows {
			return
		}
		rows := m.operationRows()
		index := m.operations.offset + y - operationHeaderRows
		if index < len(rows) {
			m.operations.cursor, m.operations.selected = index, rows[index].ID
		}
	}
}

func (m *model) renderOperations() string {
	w, h := m.operationSize()
	r := ui.FixedPromptRenderer(h, w)
	r.SetBorderTitle("Operations")
	rows := m.operationRows()
	tabs := "[Operations]  Background"
	if m.operations.background {
		tabs = " Operations  [Background]"
	}
	r.AddLines(tabs, operationCancelLabel+" "+operationClearLabel+" "+operationDetailsLabel+" "+operationCloseLabel)
	m.operations.filter.SetWidth(max(w-len("Filter: ")-common.BorderPadding, 1))
	r.AddLines("Filter: "+m.operations.filter.View(), "")
	switch {
	case m.operations.details && len(rows) > 0:
		p := rows[m.operations.cursor]
		end := time.Now()
		if !p.DoneTime.IsZero() {
			end = p.DoneTime
		}
		elapsed := end.Sub(p.Started).Round(time.Millisecond)
		control := "Ctrl+X requests cancellation at a safe checkpoint."
		if !m.processBarModel.CanCancel(p.ID) {
			control = p.CancelReason
		}
		if !p.DoneTime.IsZero() {
			control = "Finished; completed changes are retained."
		}
		text := fmt.Sprintf(
			"%s\nState: %s\nElapsed: %s\nSource: %s\nDestination: %s\nCurrent: %s\n%s\nRuns: %d\n%s\n%s\n%s",
			p.TaskName(),
			p.StatusText(),
			elapsed,
			p.Source,
			p.Destination,
			p.CurrentFile,
			operationProgress(p),
			p.Runs,
			control,
			p.Partial,
			p.ErrorMsg,
		)
		lines := strings.Split(common.MakePrintableWithEscCheck(text, false), "\n")
		limit := max(h-operationHeaderRows-common.BorderPadding, 1)
		m.operations.detailOffset = min(m.operations.detailOffset, max(len(lines)-limit, 0))
		start := m.operations.detailOffset
		for _, line := range lines[start:min(start+limit, len(lines))] {
			r.AddLines(line)
		}
	case len(rows) == 0:
		r.AddLines("No matching tasks in this session.")
	default:
		end := min(m.operations.offset+max(h-operationHeaderRows-common.BorderPadding, 1), len(rows))
		for i := m.operations.offset; i < end; i++ {
			p := rows[i]
			prefix := "  "
			if i == m.operations.cursor {
				prefix = "> "
			}
			line := fmt.Sprintf("%s%s | %s | %s", prefix, p.TaskName(), p.StatusText(), p.CurrentFile)
			r.AddLines(common.MakePrintableWithEscCheck(line, false))
		}
	}
	r.SetBorderInfoItems("Tab: switch | Enter: details | Ctrl+X: cancel | Ctrl+L: clear | Esc: back",
		fmt.Sprintf("%d task(s)", len(rows)))
	return r.Render()
}

func operationProgress(p processbar.Process) string {
	if p.Total > 0 {
		return fmt.Sprintf("Completed: %d / %d items", p.Done, p.Total)
	}
	return fmt.Sprintf("Completed: %d items; total unknown", p.Done)
}

func (m *model) trackTask(kind, source, destination string, background bool, ctx context.Context,
	cancel context.CancelFunc, cmd tea.Cmd,
) tea.Cmd {
	group := ""
	if background {
		group = kind + ":" + strconv.Itoa(m.fileModel.FocusedPanelIndex)
	}
	return m.processBarModel.Track(processbar.TaskSpec{
		Kind: kind, Source: source, Destination: destination, Background: background, GroupKey: group,
	}, ctx, cancel, cmd, operationResultError)
}

func operationResultError(msg tea.Msg) error {
	switch msg := msg.(type) {
	case MetadataMsg:
		return msg.err
	case duplicateScanMsg:
		return msg.err
	case directoryCompareMsg:
		return msg.err
	case directorySyncMsg:
		return msg.err
	case remoteOperationMsg:
		return msg.err
	case pluginCommandFinishedMsg:
		return sanitizePluginCommandError(msg.err)
	case gitStatusUpdateMsg:
		return errors.Join(msg.errors...)
	case gitOperationMsg:
		return msg.err
	case metadataOperationMsg:
		return msg.err
	case batchRenameOperationMsg:
		return msg.err
	case archiveOperationMsg:
		return msg.err
	case folderWatchSyncMsg:
		return msg.err
	case UndoOperationMsg:
		return msg.err
	default:
		return nil
	}
}

func (msg directorySyncMsg) TaskProgress() (int, int, string) {
	return len(
			msg.result.Created,
		), len(
			msg.result.Actions,
		), fmt.Sprintf(
			"Created %d item(s); completed entries are retained.\n%s",
			len(msg.result.Created),
			strings.Join(msg.result.Created, "\n"),
		)
}

func (msg duplicateScanMsg) TaskProgress() (int, int, string) {
	partial := ""
	if msg.result.Truncated || len(msg.result.Warnings) > 0 {
		partial = "Scan has limits or warnings; inspect the duplicate report."
	}
	return msg.result.FilesScanned, 0, partial
}

func (msg directoryCompareMsg) TaskProgress() (int, int, string) {
	if msg.copyPlan != nil {
		return len(msg.copyPlan.Actions), 0, "Preview only; no files changed."
	}
	partial := ""
	if len(msg.result.Warnings) > 0 {
		partial = "Comparison contains warnings; inspect its report."
	}
	return len(msg.result.Entries), 0, partial
}

func (msg remoteOperationMsg) TaskProgress() (int, int, string) {
	if msg.kind == remoteDownloadFile && msg.err == nil {
		return 1, 1, "Downloaded to " + msg.destination
	}
	return len(msg.remotes) + len(msg.entries), 0, ""
}
