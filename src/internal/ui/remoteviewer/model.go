// Package remoteviewer presents configured rclone remotes and their read-only
// directory listings. Network and filesystem operations remain in the root
// model; this package only turns explicit UI input into actions.
package remoteviewer

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/yorukot/superfile/src/internal/i18n"
	"github.com/yorukot/superfile/src/internal/remote"
	"github.com/yorukot/superfile/src/internal/ui"
)

const (
	minimumWidth     = 40
	minimumHeight    = 8
	headerLineCount  = 3
	entryStartRow    = 4
	actionRow        = 2
	frameBorderRows  = 2
	doubleClickDelay = 500 * time.Millisecond
)

// ActionKind describes an explicit request for the root model.
type ActionKind uint8

const (
	NoAction ActionKind = iota
	BrowseAction
	DownloadAction
	LoadRemotesAction
	CloseAction
)

// Action never performs I/O. The root model validates it and runs the operation.
type Action struct {
	Kind  ActionKind
	Path  remote.RemotePath
	Entry remote.Entry
}

type viewKind uint8

const (
	remotesView viewKind = iota
	entriesView
)

// Model is the state of the remote browser modal.
type Model struct {
	open        bool
	view        viewKind
	remotes     []remote.Remote
	entries     []remote.Entry
	location    remote.RemotePath
	loading     bool
	errorText   string
	notice      string
	cursor      int
	renderIndex int
	width       int
	height      int
	lastItem    string
	lastClick   time.Time
}

func New() Model { return Model{width: minimumWidth, height: minimumHeight} }

// Open starts at the configured-remotes view in a loading state.
func (m *Model) Open() {
	m.open = true
	m.view = remotesView
	m.remotes = nil
	m.entries = nil
	m.location = remote.RemotePath{}
	m.loading = true
	m.errorText = ""
	m.notice = ""
	m.resetCursor()
}

func (m *Model) Close() {
	m.open = false
	m.loading = false
	m.errorText = ""
	m.notice = ""
	m.lastItem = ""
	m.lastClick = time.Time{}
}

func (m *Model) IsOpen() bool    { return m.open }
func (m *Model) GetWidth() int   { return m.width }
func (m *Model) GetHeight() int  { return m.height }
func (m *Model) IsLoading() bool { return m.loading }

func (m *Model) SetDimensions(width, height int) {
	m.width = max(width, minimumWidth)
	m.height = max(height, minimumHeight)
	m.ensureVisible()
}

// BeginRemotes disables actions while a fresh configured-remotes request runs.
func (m *Model) BeginRemotes() {
	m.view = remotesView
	m.location = remote.RemotePath{}
	m.loading = true
	m.errorText = ""
	m.notice = ""
	m.resetCursor()
}

// BeginBrowse disables actions while a remote directory request runs.
func (m *Model) BeginBrowse(location remote.RemotePath) {
	m.view = entriesView
	m.location = location
	m.loading = true
	m.errorText = ""
	m.notice = ""
	m.resetCursor()
}

// BeginDownload keeps the current listing but prevents duplicate download
// actions until the root model reports completion.
func (m *Model) BeginDownload() {
	m.loading = true
	m.errorText = ""
	m.notice = ""
}

func (m *Model) ShowRemotes(remotes []remote.Remote) {
	m.view = remotesView
	m.location = remote.RemotePath{}
	m.remotes = append([]remote.Remote(nil), remotes...)
	m.entries = nil
	m.loading = false
	m.errorText = ""
	m.notice = ""
	m.resetCursor()
}

func (m *Model) ShowEntries(location remote.RemotePath, entries []remote.Entry) {
	m.view = entriesView
	m.location = location
	m.entries = append([]remote.Entry(nil), entries...)
	m.loading = false
	m.errorText = ""
	m.notice = ""
	m.resetCursor()
}

// ShowError leaves the current location visible so Back and Retry remain useful.
func (m *Model) ShowError(message string) {
	m.loading = false
	m.errorText = strings.TrimSpace(message)
	m.notice = ""
}

func (m *Model) ShowNotice(message string) {
	m.loading = false
	m.notice = strings.TrimSpace(message)
	m.errorText = ""
}

func (m *Model) Move(delta int) {
	if m.loading || m.itemCount() == 0 {
		return
	}
	m.cursor = min(max(m.cursor+delta, 0), m.itemCount()-1)
	m.ensureVisible()
}

// Enter opens a remote/folder and deliberately downloads a focused file.
func (m *Model) Enter() Action {
	if m.loading {
		return Action{}
	}
	if m.view == remotesView {
		item, ok := m.focusedRemote()
		if !ok {
			return Action{}
		}
		return Action{Kind: BrowseAction, Path: remote.RemotePath{Remote: item.Name}}
	}
	entry, ok := m.focusedEntry()
	if !ok {
		return Action{}
	}
	if entry.IsDir {
		return Action{Kind: BrowseAction, Path: remote.RemotePath{Remote: entry.Remote, Path: entry.Path}}
	}
	return Action{Kind: DownloadAction, Entry: entry}
}

func (m *Model) DownloadFocused() Action {
	if m.loading || m.view != entriesView {
		return Action{}
	}
	entry, ok := m.focusedEntry()
	if !ok || entry.IsDir {
		return Action{}
	}
	return Action{Kind: DownloadAction, Entry: entry}
}

func (m *Model) Back() Action {
	if m.loading {
		return Action{}
	}
	if m.view == remotesView {
		return Action{Kind: CloseAction}
	}
	if m.location.Path == "" {
		return Action{Kind: LoadRemotesAction}
	}
	parent := path.Dir(m.location.Path)
	if parent == "." {
		parent = ""
	}
	return Action{Kind: BrowseAction, Path: remote.RemotePath{Remote: m.location.Remote, Path: parent}}
}

func (m *Model) Retry() Action {
	if m.loading {
		return Action{}
	}
	if m.view == remotesView {
		return Action{Kind: LoadRemotesAction}
	}
	return Action{Kind: BrowseAction, Path: m.location}
}

func (m *Model) HandleClick(localX, localY int, now time.Time) Action {
	if !m.open || localX <= 0 || localX >= m.width-1 || localY <= 0 || localY >= m.height-1 {
		return Action{}
	}
	if localY == actionRow {
		return m.handleActionClick(localX)
	}
	if m.loading || m.errorText != "" {
		return Action{}
	}
	index := m.renderIndex + localY - entryStartRow
	if index < 0 || index >= m.itemCount() {
		return Action{}
	}
	m.cursor = index
	itemID := m.focusedID()
	if itemID == m.lastItem && !m.lastClick.IsZero() && now.Sub(m.lastClick) <= doubleClickDelay {
		m.lastItem = ""
		m.lastClick = time.Time{}
		return m.Enter()
	}
	m.lastItem = itemID
	m.lastClick = now
	return Action{}
}

func (m *Model) handleActionClick(localX int) Action {
	for _, hit := range actionHits() {
		if localX < hit.start || localX > hit.end {
			continue
		}
		switch hit.kind {
		case BrowseAction:
			return m.Enter()
		case DownloadAction:
			return m.DownloadFocused()
		case LoadRemotesAction:
			if m.errorText != "" {
				return m.Retry()
			}
			return m.Back()
		case CloseAction:
			return Action{Kind: CloseAction}
		case NoAction:
			return Action{}
		}
	}
	return Action{}
}

func (m *Model) Render() string {
	renderer := ui.PromptRenderer(m.height, m.width)
	renderer.SetBorderTitle(i18n.T("remote_files"))
	location := i18n.T("configured_remotes")
	if m.view == entriesView {
		location = m.location.String()
	}
	renderer.AddLines(" " + location)
	renderer.AddLines(actionLine())
	renderer.AddLines("")

	lines := m.contentLines()
	for _, line := range lines {
		renderer.AddLines(line)
	}
	for index := len(lines); index < m.listHeight(); index++ {
		renderer.AddLines("")
	}
	info := fmt.Sprintf("%d/%d", focusedNumber(m.cursor, m.itemCount()), m.itemCount())
	if m.notice != "" {
		info = m.notice + "  " + info
	}
	renderer.SetBorderInfoItems(i18n.T("double_click_open"), info)
	return renderer.Render()
}

func (m *Model) contentLines() []string {
	if m.loading {
		return []string{"   " + i18n.T("loading")}
	}
	if m.errorText != "" {
		return []string{" ! " + m.errorText}
	}
	if m.itemCount() == 0 {
		return []string{"   " + i18n.T("empty_remote_view")}
	}
	visible := m.visibleItems()
	lines := make([]string, 0, visible)
	for index := range visible {
		itemIndex := m.renderIndex + index
		prefix := "   "
		if itemIndex == m.cursor {
			prefix = " > "
		}
		if m.view == remotesView {
			item := m.remotes[itemIndex]
			label := item.Name + ":"
			if item.Type != "" {
				label += "  [" + item.Type + "]"
			}
			lines = append(lines, prefix+label)
			continue
		}
		entry := m.entries[itemIndex]
		label := entry.Name
		if entry.IsDir {
			label += "/"
		} else {
			label += fmt.Sprintf("  (%d B)", entry.Size)
		}
		lines = append(lines, prefix+label)
	}
	return lines
}

func (m *Model) itemCount() int {
	if m.view == remotesView {
		return len(m.remotes)
	}
	return len(m.entries)
}

func (m *Model) focusedRemote() (remote.Remote, bool) {
	if m.cursor < 0 || m.cursor >= len(m.remotes) {
		return remote.Remote{}, false
	}
	return m.remotes[m.cursor], true
}

func (m *Model) focusedEntry() (remote.Entry, bool) {
	if m.cursor < 0 || m.cursor >= len(m.entries) {
		return remote.Entry{}, false
	}
	return m.entries[m.cursor], true
}

func (m *Model) focusedID() string {
	if m.view == remotesView {
		item, ok := m.focusedRemote()
		if !ok {
			return ""
		}
		return "remote:" + item.Name
	}
	item, ok := m.focusedEntry()
	if !ok {
		return ""
	}
	return "entry:" + item.Remote + ":" + item.Path
}

func (m *Model) listHeight() int { return max(m.height-headerLineCount-frameBorderRows, 1) }

func (m *Model) visibleItems() int {
	return min(m.listHeight(), max(m.itemCount()-m.renderIndex, 0))
}

func (m *Model) resetCursor() {
	m.cursor = 0
	m.renderIndex = 0
	m.lastItem = ""
	m.lastClick = time.Time{}
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

func actionLine() string {
	labels := []string{i18n.T("open"), i18n.T("download"), i18n.T("back_retry"), i18n.T("close")}
	return " [" + strings.Join(labels, "] [") + "]"
}

func actionHits() []actionHit {
	labels := []struct {
		text string
		kind ActionKind
	}{
		{text: i18n.T("open"), kind: BrowseAction},
		{text: i18n.T("download"), kind: DownloadAction},
		{text: i18n.T("back_retry"), kind: LoadRemotesAction},
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
