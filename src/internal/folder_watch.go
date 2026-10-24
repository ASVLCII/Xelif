package internal

import (
	"context"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/folderwatch"
	"github.com/yorukot/superfile/src/internal/ui/processbar"
	"github.com/yorukot/superfile/src/pkg/utils"
)

type folderWatchEventMsg struct {
	event  folderwatch.Event
	closed bool
}

type folderWatchErrorMsg struct {
	err    error
	closed bool
}

type folderWatchSyncMsg struct{ err error }

func (m *model) startFolderWatcher() error {
	watcher, err := folderwatch.New(context.Background())
	if err != nil {
		return err
	}
	m.folderWatcher = watcher
	m.processBarModel.StartTask(
		processbar.TaskSpec{
			Kind:         "Filesystem watcher",
			Background:   true,
			Service:      true,
			CancelReason: "Required for automatic refresh; stops with Xelif.",
		},
		nil,
		nil,
	)
	return nil
}

func (m *model) currentFolderWatchPaths() []string {
	unique := make(map[string]string, len(m.fileModel.FilePanels))
	for index := range m.fileModel.FilePanels {
		path := filepath.Clean(m.fileModel.FilePanels[index].Location)
		if path == "" || path == "." {
			continue
		}
		unique[folderWatchPathKey(path)] = path
	}
	paths := make([]string, 0, len(unique))
	for _, path := range unique {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}

func (m *model) syncFolderWatcherCmd() tea.Cmd {
	if m.folderWatcher == nil {
		return nil
	}
	paths := m.currentFolderWatchPaths()
	if slices.Equal(paths, m.folderWatchPaths) {
		return nil
	}
	m.folderWatchPaths = slices.Clone(paths)
	watcher := m.folderWatcher
	return m.trackTask("Watch synchronization", strings.Join(paths, "; "), "", true, nil, nil, func() tea.Msg {
		return folderWatchSyncMsg{err: watcher.Sync(paths)}
	})
}

func (m *model) waitFolderWatchEventCmd() tea.Cmd {
	if m.folderWatcher == nil {
		return nil
	}
	events := m.folderWatcher.Events()
	return func() tea.Msg {
		event, ok := <-events
		return folderWatchEventMsg{event: event, closed: !ok}
	}
}

func (m *model) waitFolderWatchErrorCmd() tea.Cmd {
	if m.folderWatcher == nil {
		return nil
	}
	errors := m.folderWatcher.Errors()
	return func() tea.Msg {
		err, ok := <-errors
		return folderWatchErrorMsg{err: err, closed: !ok}
	}
}

func (m *model) applyFolderWatchEvent(event folderwatch.Event) {
	_ = m.processBarModel.ObserveSync(
		processbar.TaskSpec{
			Kind:       "Directory refresh",
			Source:     strings.Join(event.Paths, "; "),
			Background: true,
			GroupKey:   "directory-refresh",
		},
		func() error {
			m.refreshWatchedDirectories(event)
			return nil
		},
	)
}

func (m *model) refreshWatchedDirectories(event folderwatch.Event) {
	changed := make(map[string]struct{}, len(event.Paths))
	for _, path := range event.Paths {
		changed[folderWatchPathKey(path)] = struct{}{}
	}
	for index := range m.fileModel.FilePanels {
		panel := &m.fileModel.FilePanels[index]
		if _, ok := changed[folderWatchPathKey(panel.Location)]; ok {
			panel.UpdateElementsIfNeeded(true, m.fileModel.DisplayDotFiles)
		}
	}
}

func folderWatchPathKey(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == utils.OsWindows {
		return strings.ToLower(path)
	}
	return path
}
