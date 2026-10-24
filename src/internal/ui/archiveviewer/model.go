// Package archiveviewer presents a read-only ZIP tree for the root TUI.
package archiveviewer

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/yorukot/superfile/src/internal/archivebrowser"
	"github.com/yorukot/superfile/src/internal/i18n"
	"github.com/yorukot/superfile/src/internal/ui"
)

const (
	minimumWidth     = 36
	minimumHeight    = 8
	headerLineCount  = 3
	entryStartRow    = 4
	actionRow        = 2
	frameBorderRows  = 2
	doubleClickDelay = 500 * time.Millisecond
)

type ActionKind uint8

const (
	NoAction ActionKind = iota
	PreviewAction
	ExtractAction
	CloseAction
)

type Action struct {
	Kind   ActionKind
	Member string
}

// Operation is an immutable request snapshot. Workers must use their own
// archive handle so closing this viewer never waits for a read or extraction.
type Operation struct {
	Context     context.Context
	Cancel      context.CancelFunc
	ArchivePath string
	RequestID   uint64
}

type Model struct {
	archive         *archivebrowser.Archive
	archivePath     string
	directory       string
	entries         []archivebrowser.Entry
	open            bool
	cursor          int
	renderIndex     int
	width           int
	height          int
	lastMember      string
	lastClick       time.Time
	operationID     uint64
	operationCancel context.CancelFunc
}

func New() Model { return Model{width: minimumWidth, height: minimumHeight} }

func (m *Model) Open(archivePath string) error {
	_ = m.Close()
	archive, err := archivebrowser.Open(archivePath)
	if err != nil {
		return err
	}
	m.archive = archive
	m.archivePath = archivePath
	m.directory = ""
	m.open = true
	m.cursor = 0
	m.renderIndex = 0
	if err := m.reload(); err != nil {
		_ = m.Close()
		return err
	}
	return nil
}

func (m *Model) Close() error {
	m.cancelOperation()

	var err error
	if m.archive != nil {
		err = m.archive.Close()
	}
	m.archive = nil
	m.archivePath = ""
	m.directory = ""
	m.entries = nil
	m.open = false
	m.cursor = 0
	m.renderIndex = 0
	m.lastMember = ""
	m.lastClick = time.Time{}
	return err
}

func (m *Model) IsOpen() bool        { return m.open }
func (m *Model) GetWidth() int       { return m.width }
func (m *Model) GetHeight() int      { return m.height }
func (m *Model) ArchivePath() string { return m.archivePath }

// StartOperation cancels previous work and binds a new request to this archive.
// It is called on the UI goroutine, not from an asynchronous command.
func (m *Model) StartOperation() (Operation, error) {
	if !m.open || m.archive == nil {
		return Operation{}, archivebrowser.ErrClosed
	}
	m.cancelOperation()
	ctx, cancel := context.WithCancel(context.Background())
	m.operationCancel = cancel
	return Operation{Context: ctx, Cancel: cancel, ArchivePath: m.archivePath, RequestID: m.operationID}, nil
}

// CompleteOperation accepts the current request once. Cancelled work, replaced
// archives (even at the same path), and duplicate messages cannot change the UI.
func (m *Model) CompleteOperation(operation Operation) bool {
	if !m.open || m.operationCancel == nil || operation.RequestID != m.operationID ||
		operation.ArchivePath != m.archivePath || operation.Context == nil || operation.Context.Err() != nil {
		return false
	}
	m.operationCancel()
	m.operationCancel = nil
	return true
}

func (m *Model) cancelOperation() {
	if m.operationCancel != nil {
		m.operationCancel()
		m.operationCancel = nil
	}
	m.operationID++
}

func (m *Model) SetDimensions(width, height int) {
	m.width = max(width, minimumWidth)
	m.height = max(height, minimumHeight)
	m.ensureVisible()
}

func (m *Model) Move(delta int) {
	if len(m.entries) == 0 {
		return
	}
	m.cursor = min(max(m.cursor+delta, 0), len(m.entries)-1)
	m.ensureVisible()
}

func (m *Model) Enter() Action {
	entry, ok := m.focused()
	if !ok {
		return Action{}
	}
	if !entry.IsDir {
		return Action{Kind: PreviewAction, Member: entry.Path}
	}
	m.directory = entry.Path
	m.cursor = 0
	m.renderIndex = 0
	if err := m.reload(); err != nil {
		return Action{}
	}
	return Action{}
}

func (m *Model) Back() Action {
	if m.directory == "" {
		return Action{Kind: CloseAction}
	}
	parent := path.Dir(m.directory)
	if parent == "." {
		parent = ""
	}
	m.directory = parent
	m.cursor = 0
	m.renderIndex = 0
	_ = m.reload()
	return Action{}
}

func (m *Model) ExtractFocused() Action {
	entry, ok := m.focused()
	if !ok {
		return Action{}
	}
	return Action{Kind: ExtractAction, Member: entry.Path}
}

func (m *Model) HandleClick(localX, localY int, now time.Time) Action {
	if !m.open || localX <= 0 || localX >= m.width-1 || localY <= 0 || localY >= m.height-1 {
		return Action{}
	}
	if localY == actionRow {
		for _, hit := range archiveActionHits() {
			if localX < hit.start || localX > hit.end {
				continue
			}
			switch hit.kind {
			case PreviewAction:
				return m.Enter()
			case ExtractAction:
				return m.ExtractFocused()
			case NoAction:
				return m.Back()
			case CloseAction:
				return Action{Kind: CloseAction}
			}
		}
		return Action{}
	}
	index := m.renderIndex + localY - entryStartRow
	if index < 0 || index >= len(m.entries) {
		return Action{}
	}
	m.cursor = index
	member := m.entries[index].Path
	if member == m.lastMember && !m.lastClick.IsZero() && now.Sub(m.lastClick) <= doubleClickDelay {
		m.lastMember = ""
		m.lastClick = time.Time{}
		return m.Enter()
	}
	m.lastMember = member
	m.lastClick = now
	return Action{}
}

func (m *Model) Render() string {
	renderer := ui.PromptRenderer(m.height, m.width)
	renderer.SetBorderTitle(i18n.T("archive") + ": " + filepath.Base(m.archivePath))
	directory := "/" + m.directory
	renderer.AddLines(" " + directory)
	renderer.AddLines(archiveActionLine())
	renderer.AddLines("")
	visible := m.visibleEntries()
	for index := range visible {
		entryIndex := m.renderIndex + index
		line := "   "
		if entryIndex == m.cursor {
			line = " > "
		}
		entry := m.entries[entryIndex]
		name := entry.Name
		if entry.IsDir {
			name += "/"
		}
		renderer.AddLines(line + name)
	}
	for index := visible; index < m.listHeight(); index++ {
		renderer.AddLines("")
	}
	renderer.SetBorderInfoItems(i18n.T("double_click_open"),
		fmt.Sprintf("%d/%d", focusedNumber(m.cursor, len(m.entries)), len(m.entries)))
	return renderer.Render()
}

func (m *Model) reload() error {
	entries, err := m.archive.List(m.directory)
	if err != nil {
		return err
	}
	m.entries = entries
	if len(entries) == 0 {
		m.cursor = 0
		m.renderIndex = 0
	} else {
		m.cursor = min(m.cursor, len(entries)-1)
	}
	m.ensureVisible()
	return nil
}

func (m *Model) focused() (archivebrowser.Entry, bool) {
	if m.cursor < 0 || m.cursor >= len(m.entries) {
		return archivebrowser.Entry{}, false
	}
	return m.entries[m.cursor], true
}

func (m *Model) listHeight() int { return max(m.height-headerLineCount-frameBorderRows, 1) }

func (m *Model) visibleEntries() int {
	return min(m.listHeight(), max(len(m.entries)-m.renderIndex, 0))
}

func (m *Model) ensureVisible() {
	if m.cursor < m.renderIndex {
		m.renderIndex = m.cursor
	}
	if m.cursor >= m.renderIndex+m.listHeight() {
		m.renderIndex = m.cursor - m.listHeight() + 1
	}
	m.renderIndex = max(m.renderIndex, 0)
}

func focusedNumber(cursor, total int) int {
	if total == 0 {
		return 0
	}
	return cursor + 1
}

type actionHit struct {
	start int
	end   int
	kind  ActionKind
}

func archiveActionLine() string {
	labels := []string{i18n.T("open"), i18n.T("extract"), i18n.T("back"), i18n.T("close")}
	return " [" + strings.Join(labels, "] [") + "]"
}

func archiveActionHits() []actionHit {
	labels := []struct {
		text string
		kind ActionKind
	}{
		{text: i18n.T("open"), kind: PreviewAction},
		{text: i18n.T("extract"), kind: ExtractAction},
		{text: i18n.T("back"), kind: NoAction},
		{text: i18n.T("close"), kind: CloseAction},
	}
	position := 2
	hits := make([]actionHit, 0, len(labels))
	for _, label := range labels {
		width := ansi.StringWidth("[" + label.text + "]")
		hits = append(hits, actionHit{start: position, end: position + width - 1, kind: label.kind})
		position += width + 1
	}
	return hits
}

func IsText(data []byte) bool {
	return !bytes.ContainsRune(data, 0)
}
