package internal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/remote"
	"github.com/yorukot/superfile/src/internal/ui/remoteviewer"
)

type remoteOperationKind uint8

type remoteService interface {
	Remotes(context.Context) ([]remote.Remote, error)
	List(context.Context, remote.RemotePath) ([]remote.Entry, error)
	Download(context.Context, remote.Entry, string) (string, error)
}

const (
	remoteLoadRemotes remoteOperationKind = iota
	remoteListDirectory
	remoteDownloadFile
)

type remoteOperationMsg struct {
	reqID       int
	kind        remoteOperationKind
	location    remote.RemotePath
	remotes     []remote.Remote
	entries     []remote.Entry
	destination string
	err         error
}

func (m *model) openRemoteBrowserCmd() tea.Cmd {
	client, err := remote.New()
	if err != nil {
		m.showUserError("Remote files unavailable", errors.New(remoteUserMessage(err)))
		return nil
	}
	m.remoteClient = client
	m.remoteViewer.Open()
	m.setRemoteViewerSize()
	return m.loadRemoteListCmd()
}

func (m *model) setRemoteViewerSize() {
	m.remoteViewer.SetDimensions(3*m.fullWidth/4, 3*m.fullHeight/4) //nolint:mnd // Remote listings need space.
}

func (m *model) loadRemoteListCmd() tea.Cmd {
	if m.remoteClient == nil {
		return m.openRemoteBrowserCmd()
	}
	m.remoteViewer.BeginRemotes()
	ctx, reqID := m.nextRemoteContext()
	client := m.remoteClient
	return func() tea.Msg {
		remotes, err := client.Remotes(ctx)
		return remoteOperationMsg{reqID: reqID, kind: remoteLoadRemotes, remotes: remotes, err: err}
	}
}

func (m *model) browseRemoteCmd(location remote.RemotePath) tea.Cmd {
	if m.remoteClient == nil {
		m.showUserError("Remote files unavailable", errors.New("open the remote browser again"))
		return nil
	}
	normalized, err := remote.ParsePath(location.String())
	if err != nil {
		m.showUserError("Invalid remote path", errors.New(remoteUserMessage(err)))
		return nil
	}
	m.remoteViewer.BeginBrowse(normalized)
	ctx, reqID := m.nextRemoteContext()
	client := m.remoteClient
	return func() tea.Msg {
		entries, listErr := client.List(ctx, normalized)
		return remoteOperationMsg{
			reqID: reqID, kind: remoteListDirectory, location: normalized, entries: entries, err: listErr,
		}
	}
}

func (m *model) downloadRemoteCmd(entry remote.Entry) tea.Cmd {
	if m.remoteClient == nil {
		m.showUserError("Remote files unavailable", errors.New("open the remote browser again"))
		return nil
	}
	destination := m.getFocusedFilePanel().Location
	m.remoteViewer.BeginDownload()
	ctx, reqID := m.nextRemoteContext()
	client := m.remoteClient
	return func() tea.Msg {
		localPath, err := client.Download(ctx, entry, destination)
		return remoteOperationMsg{
			reqID: reqID, kind: remoteDownloadFile, destination: localPath, err: err,
		}
	}
}

func (m *model) nextRemoteContext() (context.Context, int) {
	if m.remoteCancel != nil {
		m.remoteCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.remoteCancel = cancel
	m.remoteReqID++
	return ctx, m.remoteReqID
}

func (m *model) closeRemoteBrowser() {
	if m.remoteCancel != nil {
		m.remoteCancel()
		m.remoteCancel = nil
	}
	m.remoteReqID++ // Ignore a completion already queued by Bubble Tea.
	m.remoteViewer.Close()
}

func (m *model) handleRemoteViewerKey(key string) tea.Cmd {
	switch {
	case slices.Contains(common.Hotkeys.Quit, key), slices.Contains(common.Hotkeys.CancelTyping, key):
		m.closeRemoteBrowser()
	case slices.Contains(common.Hotkeys.ListUp, key):
		m.remoteViewer.Move(-1)
	case slices.Contains(common.Hotkeys.ListDown, key):
		m.remoteViewer.Move(1)
	case slices.Contains(common.Hotkeys.PageUp, key):
		m.remoteViewer.Move(-viewerPageScrollStep)
	case slices.Contains(common.Hotkeys.PageDown, key):
		m.remoteViewer.Move(viewerPageScrollStep)
	case slices.Contains(common.Hotkeys.ParentDirectory, key):
		return m.handleRemoteAction(m.remoteViewer.Back())
	case slices.Contains(common.Hotkeys.Confirm, key), slices.Contains(common.Hotkeys.ConfirmTyping, key):
		return m.handleRemoteAction(m.remoteViewer.Enter())
	case key == "d":
		return m.handleRemoteAction(m.remoteViewer.DownloadFocused())
	case key == "r":
		return m.handleRemoteAction(m.remoteViewer.Retry())
	}
	return nil
}

func (m *model) handleRemoteViewerClick(mouse tea.Mouse) tea.Cmd {
	overlayX := m.fullWidth/common.CenterDivisor - m.remoteViewer.GetWidth()/common.CenterDivisor
	overlayY := m.fullHeight/common.CenterDivisor - m.remoteViewer.GetHeight()/common.CenterDivisor
	if mouse.X < overlayX || mouse.X >= overlayX+m.remoteViewer.GetWidth() ||
		mouse.Y < overlayY || mouse.Y >= overlayY+m.remoteViewer.GetHeight() {
		return nil
	}
	action := m.remoteViewer.HandleClick(mouse.X-overlayX, mouse.Y-overlayY, time.Now())
	return m.handleRemoteAction(action)
}

func (m *model) handleRemoteAction(action remoteviewer.Action) tea.Cmd {
	switch action.Kind {
	case remoteviewer.NoAction:
		return nil
	case remoteviewer.BrowseAction:
		return m.browseRemoteCmd(action.Path)
	case remoteviewer.DownloadAction:
		return m.downloadRemoteCmd(action.Entry)
	case remoteviewer.LoadRemotesAction:
		return m.loadRemoteListCmd()
	case remoteviewer.CloseAction:
		m.closeRemoteBrowser()
		return nil
	default:
		return nil
	}
}

func (m *model) applyRemoteOperation(msg remoteOperationMsg) {
	if msg.reqID != m.remoteReqID {
		return
	}
	m.remoteCancel = nil
	if msg.err != nil {
		if errors.Is(msg.err, context.Canceled) {
			return
		}
		var typedError *remote.Error
		if errors.As(msg.err, &typedError) {
			slog.Error("Remote operation failed", "operation", typedError.Op,
				"kind", typedError.Kind, "remotePath", typedError.RemotePath)
		} else {
			slog.Error("Remote operation failed")
		}
		message := remoteUserMessage(msg.err)
		m.remoteViewer.ShowError(message)
		m.showUserError("Remote operation failed", errors.New(message))
		return
	}
	switch msg.kind {
	case remoteLoadRemotes:
		m.remoteViewer.ShowRemotes(msg.remotes)
	case remoteListDirectory:
		m.remoteViewer.ShowEntries(msg.location, msg.entries)
	case remoteDownloadFile:
		m.remoteViewer.ShowNotice("Downloaded " + filepath.Base(msg.destination))
		m.fileModel.UpdateFilePanelsIfNeeded(true)
	}
}

func remoteUserMessage(err error) string {
	var remoteErr *remote.Error
	if !errors.As(err, &remoteErr) {
		return "The remote operation failed. See the Xelif log for details."
	}
	switch remoteErr.Kind {
	case remote.Unavailable:
		return "rclone is not installed or is not available on PATH."
	case remote.NotConfigured:
		return "That rclone remote is not configured. Configure it with rclone, then retry."
	case remote.Authentication:
		return "Remote authentication failed. Update the remote with rclone, then retry."
	case remote.TimedOut:
		return "The remote operation timed out."
	case remote.Cancelled:
		return "The remote operation was cancelled."
	case remote.OutputLimit:
		return "The remote returned too many entries for one bounded request."
	case remote.InvalidOutput:
		return "rclone returned an invalid directory listing."
	case remote.InvalidPath:
		return "The remote path is invalid."
	case remote.AlreadyExists:
		return "A local file with that name already exists. Xelif did not overwrite it."
	case remote.CommandFailed:
		return "rclone could not complete the operation. See the Xelif log for details."
	default:
		return fmt.Sprintf("The remote operation failed (%s).", remoteErr.Kind)
	}
}
