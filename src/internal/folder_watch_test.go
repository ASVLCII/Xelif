package internal

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/folderwatch"
)

type fakeFolderWatcher struct {
	syncCalls [][]string
	syncErr   error
	events    chan folderwatch.Event
	errors    chan error
	closed    bool
}

func newFakeFolderWatcher() *fakeFolderWatcher {
	return &fakeFolderWatcher{
		events: make(chan folderwatch.Event),
		errors: make(chan error),
	}
}

func (f *fakeFolderWatcher) Sync(paths []string) error {
	f.syncCalls = append(f.syncCalls, slices.Clone(paths))
	return f.syncErr
}

func (f *fakeFolderWatcher) Events() <-chan folderwatch.Event { return f.events }
func (f *fakeFolderWatcher) Errors() <-chan error             { return f.errors }
func (f *fakeFolderWatcher) Close() error {
	f.closed = true
	return nil
}

func TestFolderWatcherSyncsOnlyWhenVisibleLocationsChange(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	m := defaultTestModel(first, second)
	fake := newFakeFolderWatcher()
	m.folderWatcher = fake

	cmd := m.syncFolderWatcherCmd()
	require.NotNil(t, cmd)
	msg := cmd()
	require.IsType(t, folderWatchSyncMsg{}, msg)
	require.Len(t, fake.syncCalls, 1)
	assert.ElementsMatch(t, []string{first, second}, fake.syncCalls[0])
	assert.Nil(t, m.syncFolderWatcherCmd())

	require.NoError(t, m.updateCurrentFilePanelDir(t.TempDir()))
	assert.NotNil(t, m.syncFolderWatcherCmd())
}

func TestFolderWatchEventRefreshesOnlyMatchingPanel(t *testing.T) {
	directory := t.TempDir()
	m := defaultTestModel(directory)
	panel := m.getFocusedFilePanel()
	assert.Equal(t, -1, panel.FindElementIndexByName("new.txt"))

	require.NoError(t, os.WriteFile(filepath.Join(directory, "new.txt"), []byte("new"), 0o600))
	m.applyFolderWatchEvent(folderwatch.Event{Paths: []string{directory}})

	assert.NotEqual(t, -1, panel.FindElementIndexByName("new.txt"))
}

func TestFolderWatchSyncErrorIsReportedWithoutDisablingPolling(t *testing.T) {
	directory := t.TempDir()
	m := defaultTestModel(directory)
	fake := newFakeFolderWatcher()
	fake.syncErr = errors.New("watch limit reached")
	m.folderWatcher = fake

	cmd := m.syncFolderWatcherCmd()
	require.NotNil(t, cmd)
	msg := cmd().(folderWatchSyncMsg)
	require.ErrorContains(t, msg.err, "watch limit reached")
	assert.NotNil(t, m.fileModel.FilePanels)
}
