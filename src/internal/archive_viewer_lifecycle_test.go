package internal

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/ui/archiveviewer"
)

func TestArchiveQueuedOperationDoesNotReadReplacementArchive(t *testing.T) {
	first := writeLifecycleArchive(t, map[string]string{"shared.txt": "first archive"}, zip.Store)
	second := writeLifecycleArchive(t, map[string]string{"shared.txt": "second archive"}, zip.Store)
	m := defaultTestModel(t.TempDir())
	require.NoError(t, m.openArchive(first))
	t.Cleanup(func() { _ = m.archiveViewer.Close() })
	cmd := m.handleArchiveAction(archiveviewer.Action{Kind: archiveviewer.PreviewAction, Member: "shared.txt"})
	require.NotNil(t, cmd)

	require.NoError(t, m.openArchive(second))
	msg := cmd().(archiveOperationMsg)
	require.ErrorIs(t, msg.err, context.Canceled)
	assert.Empty(t, msg.content)
	m.applyArchiveOperation(msg)
	assert.False(t, m.textViewer.IsOpen())
	assert.False(t, m.notifyModel.IsOpen(), "cancelled old work must not show an error over the new archive")
}

func TestArchiveQueuedExtractionIsCancelledBeforeWriting(t *testing.T) {
	archivePath := writeLifecycleArchive(t, map[string]string{"member.txt": "archive content"}, zip.Store)
	destination := t.TempDir()
	m := defaultTestModel(destination)
	require.NoError(t, m.openArchive(archivePath))
	cmd := m.handleArchiveAction(archiveviewer.Action{Kind: archiveviewer.ExtractAction, Member: "member.txt"})
	require.NotNil(t, cmd)
	require.NoError(t, m.archiveViewer.Close())

	msg := cmd().(archiveOperationMsg)
	require.ErrorIs(t, msg.err, context.Canceled)
	_, err := os.Stat(filepath.Join(destination, "member.txt"))
	require.ErrorIs(t, err, os.ErrNotExist)
	m.applyArchiveOperation(msg)
	assert.False(t, m.textViewer.IsOpen())
	assert.False(t, m.notifyModel.IsOpen())
}

func TestArchiveCompletionIsIgnoredAfterReopeningSameArchive(t *testing.T) {
	archivePath := writeLifecycleArchive(t, map[string]string{"member.txt": "old result"}, zip.Store)
	m := defaultTestModel(t.TempDir())
	require.NoError(t, m.openArchive(archivePath))
	t.Cleanup(func() { _ = m.archiveViewer.Close() })
	cmd := m.handleArchiveAction(archiveviewer.Action{Kind: archiveviewer.PreviewAction, Member: "member.txt"})
	msg := cmd().(archiveOperationMsg)
	require.NoError(t, msg.err)
	require.NoError(t, m.openArchive(archivePath))
	m.textViewer.Open("Keep current view", "untouched")
	before := m.textViewer.Render()

	m.applyArchiveOperation(msg)
	assert.Equal(t, before, m.textViewer.Render())

	msg.err = os.ErrPermission
	m.applyArchiveOperation(msg)
	assert.False(t, m.notifyModel.IsOpen(), "stale failures must also be ignored")
}

func TestArchiveReplacementOperationIgnoresPreviousCompletion(t *testing.T) {
	archivePath := writeLifecycleArchive(t, map[string]string{"first.txt": "first", "second.txt": "second"}, zip.Store)
	m := defaultTestModel(t.TempDir())
	require.NoError(t, m.openArchive(archivePath))
	t.Cleanup(func() { _ = m.archiveViewer.Close() })
	first := m.handleArchiveAction(archiveviewer.Action{Kind: archiveviewer.PreviewAction, Member: "first.txt"})()
	secondCmd := m.handleArchiveAction(archiveviewer.Action{Kind: archiveviewer.PreviewAction, Member: "second.txt"})

	m.applyArchiveOperation(first.(archiveOperationMsg))
	assert.False(t, m.textViewer.IsOpen())
	second := secondCmd().(archiveOperationMsg)
	require.NoError(t, second.err)
	m.applyArchiveOperation(second)
	assert.True(t, m.textViewer.IsOpen())
	assert.Contains(t, m.textViewer.Render(), "second")
	m.textViewer.Close()
	m.applyArchiveOperation(second)
	assert.False(t, m.textViewer.IsOpen(), "a completion is accepted at most once")
}

func TestArchiveExtractionKeepsCapturedDestinationAndDoesNotOverwrite(t *testing.T) {
	archivePath := writeLifecycleArchive(t, map[string]string{"member.txt": "archive content"}, zip.Store)
	destination := t.TempDir()
	otherDirectory := t.TempDir()
	m := defaultTestModel(destination)
	require.NoError(t, m.openArchive(archivePath))
	t.Cleanup(func() { _ = m.archiveViewer.Close() })
	cmd := m.handleArchiveAction(archiveviewer.Action{Kind: archiveviewer.ExtractAction, Member: "member.txt"})
	require.NoError(t, m.updateCurrentFilePanelDir(otherDirectory))

	msg := cmd().(archiveOperationMsg)
	require.NoError(t, msg.err)
	content, err := os.ReadFile(filepath.Join(destination, "member.txt"))
	require.NoError(t, err)
	assert.Equal(t, "archive content", string(content))
	_, err = os.Stat(filepath.Join(otherDirectory, "member.txt"))
	require.ErrorIs(t, err, os.ErrNotExist)

	require.NoError(t, m.updateCurrentFilePanelDir(destination))
	cmd = m.handleArchiveAction(archiveviewer.Action{Kind: archiveviewer.ExtractAction, Member: "member.txt"})
	msg = cmd().(archiveOperationMsg)
	require.Error(t, msg.err)
	content, err = os.ReadFile(filepath.Join(destination, "member.txt"))
	require.NoError(t, err)
	assert.Equal(t, "archive content", string(content))
}

func TestArchiveCurrentErrorIsVisibleAndEscapeReturnsToPicker(t *testing.T) {
	archivePath := writeLifecycleArchive(t, map[string]string{"folder/member.txt": "archive content"}, zip.Store)
	destination := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(destination, "folder"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(destination, "folder", "member.txt"), []byte("keep existing"), 0o600))
	m := defaultTestModel(destination)
	require.NoError(t, m.openArchive(archivePath))
	t.Cleanup(func() { _ = m.archiveViewer.Close() })
	m.archiveViewer.Enter()
	before := m.archiveViewer.Render()
	cmd := m.handleArchiveAction(m.archiveViewer.ExtractFocused())
	require.NotNil(t, cmd)
	msg := cmd().(archiveOperationMsg)
	require.Error(t, msg.err)

	m.applyArchiveOperation(msg)
	assert.True(t, m.archiveViewer.IsOpen(), "failure must preserve browsing context")
	assert.True(t, m.textViewer.IsOpen())
	assert.False(t, m.notifyModel.IsOpen(), "the archive picker must not conceal an error notification")
	displayed := ansi.Strip(m.updateRenderForOverlay(m.mainComponentsRender()))
	assert.Contains(t, displayed, "Archive operation failed")

	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, m.textViewer.IsOpen())
	assert.True(t, m.archiveViewer.IsOpen())
	assert.Equal(t, before, m.archiveViewer.Render(), "Escape must return to the same directory and selection")
	displayed = ansi.Strip(m.updateRenderForOverlay(m.mainComponentsRender()))
	assert.Contains(t, displayed, "Archive: lifecycle.zip")
	assert.NotContains(t, displayed, "Archive operation failed")
	assert.False(t, m.notifyModel.IsOpen())
}

const lifecycleZIPMethod uint16 = 0xfffd

// archive/zip's custom decompressor registration is process-wide, including repeated test runs.
var lifecycleDecompressorOnce sync.Once //nolint:gochecknoglobals // Register the test-only ZIP method exactly once.

var lifecycleReadGates sync.Map //nolint:gochecknoglobals // Concurrent gates for the process-wide test decompressor.

type lifecycleReadGate struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

type lifecycleBlockedReader struct {
	source io.Reader
	gate   *lifecycleReadGate
}

func (reader *lifecycleBlockedReader) Read(buffer []byte) (int, error) {
	reader.gate.once.Do(func() {
		close(reader.gate.started)
		<-reader.gate.release
	})
	return reader.source.Read(buffer)
}

type lifecycleStoredWriter struct{ io.Writer }

func (lifecycleStoredWriter) Close() error { return nil }

func TestArchiveCloseDoesNotWaitForActiveReadOrExtraction(t *testing.T) {
	lifecycleDecompressorOnce.Do(func() {
		zip.RegisterDecompressor(lifecycleZIPMethod, func(source io.Reader) io.ReadCloser {
			content, err := io.ReadAll(source)
			if err != nil {
				return io.NopCloser(source)
			}
			reader := io.Reader(bytes.NewReader(content))
			if gate, ok := lifecycleReadGates.Load(string(content)); ok {
				reader = &lifecycleBlockedReader{source: reader, gate: gate.(*lifecycleReadGate)}
			}
			return io.NopCloser(reader)
		})
	})
	for _, scenario := range []struct {
		kind archiveviewer.ActionKind
		name string
	}{{archiveviewer.PreviewAction, "read"}, {archiveviewer.ExtractAction, "extract"}} {
		kind := scenario.kind
		t.Run(scenario.name, func(t *testing.T) {
			content := strings.Repeat("blocked archive member\n", 4096) + t.TempDir()
			gate := &lifecycleReadGate{started: make(chan struct{}), release: make(chan struct{})}
			lifecycleReadGates.Store(content, gate)
			t.Cleanup(func() { lifecycleReadGates.Delete(content) })
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(gate.release) }) }
			t.Cleanup(release)
			archivePath := writeLifecycleArchive(t, map[string]string{"member.txt": content}, lifecycleZIPMethod)
			destination := t.TempDir()
			m := defaultTestModel(destination)
			require.NoError(t, m.openArchive(archivePath))
			cmd := m.handleArchiveAction(archiveviewer.Action{Kind: kind, Member: "member.txt"})
			result := make(chan archiveOperationMsg, 1)
			go func() { result <- cmd().(archiveOperationMsg) }()
			select {
			case <-gate.started:
			case <-time.After(time.Second):
				t.Fatal("archive worker did not begin reading")
			}
			closed := make(chan error, 1)
			go func() { closed <- m.archiveViewer.Close() }()
			select {
			case err := <-closed:
				require.NoError(t, err)
			case <-time.After(time.Second):
				t.Error("closing the UI waited for the worker's archive handle")
			}
			release()
			select {
			case msg := <-result:
				require.ErrorIs(t, msg.err, context.Canceled)
				m.applyArchiveOperation(msg)
			case <-time.After(time.Second):
				t.Fatal("cancelled archive worker did not finish")
			}
			assert.False(t, m.textViewer.IsOpen())
			assert.False(t, m.notifyModel.IsOpen())
			if kind == archiveviewer.ExtractAction {
				_, err := os.Stat(filepath.Join(destination, "member.txt"))
				require.ErrorIs(t, err, os.ErrNotExist, "cancelled extraction must remove its partial file")
			}
		})
	}
}

func writeLifecycleArchive(t *testing.T, members map[string]string, method uint16) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "lifecycle.zip")
	file, err := os.Create(archivePath)
	require.NoError(t, err)
	writer := zip.NewWriter(file)
	if method == lifecycleZIPMethod {
		writer.RegisterCompressor(method, func(destination io.Writer) (io.WriteCloser, error) {
			return lifecycleStoredWriter{Writer: destination}, nil
		})
	}
	for name, content := range members {
		member, createErr := writer.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		require.NoError(t, createErr)
		_, writeErr := io.WriteString(member, content)
		require.NoError(t, writeErr)
	}
	require.NoError(t, writer.Close())
	require.NoError(t, file.Close())
	return archivePath
}
