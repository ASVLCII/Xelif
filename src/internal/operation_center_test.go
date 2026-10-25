package internal

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/ui/commandpalette"
	"github.com/yorukot/superfile/src/internal/ui/processbar"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestOperationShutdownUnblocksQueuedFileProcessor(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	t.Cleanup(m.processBarModel.Shutdown)
	// A prior file error owns this lock until its dialog is dismissed. A queued
	// worker must still be able to stop when shutdown no longer dispatches dialogs.
	m.mutexErrorModal.Lock()
	defer m.mutexErrorModal.Unlock()
	m.processBarModel.Shutdown()
	finished := make(chan processbar.ProcessState, 1)
	go func() {
		m.runFileProcessor(func([]string) (processbar.Process, []string) {
			return processbar.Process{State: processbar.Successful}, nil
		}, func(state processbar.ProcessState, _ int) tea.Msg {
			finished <- state
			return nil
		}, nil, 1)
	}()
	select {
	case state := <-finished:
		require.Equal(t, processbar.Cancelled, state)
	case <-time.After(time.Second):
		t.Fatal("shutdown stranded a queued file processor behind an error dialog")
	}
}

func TestOperationShutdownInterruptsQueuedFileProcessor(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	t.Cleanup(m.processBarModel.Shutdown)
	m.mutexErrorModal.Lock()
	defer m.mutexErrorModal.Unlock()
	entered, finished := make(chan struct{}), make(chan bool, 1)
	cmd := m.processBarModel.Run(func() tea.Msg {
		close(entered)
		acquired := m.lockFileProcessor()
		if acquired {
			m.mutexErrorModal.Unlock()
		}
		finished <- acquired
		return nil
	})
	go cmd()
	<-entered
	m.processBarModel.Shutdown()
	select {
	case acquired := <-finished:
		require.False(t, acquired, "a queued processor must not start during shutdown")
	case <-time.After(time.Second):
		t.Fatal("shutdown did not interrupt the queued worker")
	}
	m.processBarModel.WaitWorkers()
}

func TestOperationCenterDispatchAndInputIsolation(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	t.Cleanup(m.processBarModel.Shutdown)
	m.applyCommandPaletteAction(commandpalette.ExecuteAction{ID: commandIDOperations})
	require.True(t, m.operations.open)
	require.True(t, m.IsOverlayModelOpen())
	before := m.getFocusedFilePanel().Location
	TeaUpdate(m, utils.TeaRuneKeyMsg("q"))
	require.True(t, m.operations.open, "typing q filters, rather than quitting Xelif")
	require.Equal(t, "q", m.operations.filter.Value())
	require.Equal(t, before, m.getFocusedFilePanel().Location)
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	require.False(t, m.operations.open)
}

func TestOperationCenterFooterClickAndBackgroundTabs(t *testing.T) {
	m := defaultTestModelWithFooterAndFilePreview(t.TempDir())
	t.Cleanup(m.processBarModel.Shutdown)
	m.firstLoadingComplete = true
	m.fileModel.FilePreview.Close()
	m.handleLeftMouseClick(tea.Mouse{X: 1, Y: m.fullHeight - 2})
	require.True(t, m.operations.open)
	ctx, cancel := context.WithCancel(context.Background())
	p := m.processBarModel.StartTask(processbar.TaskSpec{Kind: "Visible copy", Source: "hello"}, ctx, cancel)
	require.Len(t, m.operationRows(), 1)
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyTab})
	require.True(t, m.operations.background)
	require.Contains(t, ansi.Strip(m.renderOperations()), "Progress listener")
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyTab})
	TeaUpdate(m, utils.TeaRuneKeyMsg("ctrl+x"))
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	stored, _ := m.processBarModel.GetByID(p.ID)
	require.Equal(t, "Cancel requested", stored.StatusText())
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	require.False(t, m.operations.open)
	require.Equal(t, processbar.InOperation, stored.State)
}

func TestOperationCenterSelectionDetailsFilterAndResize(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	t.Cleanup(m.processBarModel.Shutdown)
	for _, kind := range []string{"Alpha", "Beta", "Gamma"} {
		m.processBarModel.StartTask(processbar.TaskSpec{Kind: kind, Source: "source\x1b[31m"}, nil, nil)
	}
	m.openOperations()
	m.moveOperation(1)
	selected := m.operations.selected
	m.processBarModel.StartTask(processbar.TaskSpec{Kind: "Delta"}, nil, nil)
	m.operationRows()
	require.Equal(t, selected, m.operations.selected)
	TeaUpdate(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	rendered := ansi.Strip(m.renderOperations())
	require.Contains(t, rendered, "State: Running")
	require.Contains(t, rendered, "total unknown")
	for _, size := range [][2]int{{80, 24}, {54, 16}, {30, 12}} {
		m.fullWidth, m.fullHeight = size[0], size[1]
		w, h := m.operationSize()
		rendered := ansi.Strip(m.renderOperations())
		require.Len(t, strings.Split(rendered, "\n"), h)
		for _, line := range strings.Split(rendered, "\n") {
			require.LessOrEqual(t, ansi.StringWidth(line), w)
		}
	}
}

func TestOperationCenterPasteFiltersWithoutDroppingPaths(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	t.Cleanup(m.processBarModel.Shutdown)
	m.processBarModel.StartTask(processbar.TaskSpec{Kind: "Copy task", Source: "example"}, nil, nil)
	m.openOperations()
	TeaUpdate(m, tea.PasteMsg{Content: "Copy task"})
	require.Equal(t, "Copy task", m.operations.filter.Value())
	require.Len(t, m.operationRows(), 1)
	require.False(t, m.notifyModel.IsOpen())
}

func TestOperationCompressionFailureNeverPublishes(t *testing.T) {
	dir := t.TempDir()
	processes := processbar.New()
	t.Cleanup(processes.Shutdown)
	processes.ListenForChannelUpdates()
	target := filepath.Join(dir, "failed.zip")
	err := zipSources([]string{filepath.Join(dir, "missing")}, 0, target, &processes)
	require.Error(t, err)
	require.NoFileExists(t, target)
	matches, err := filepath.Glob(filepath.Join(dir, ".xelif-archive-*"))
	require.NoError(t, err)
	require.Empty(t, matches)
}

func TestOperationCompressionExistingDestinationPreserved(t *testing.T) {
	dir := t.TempDir()
	processes := processbar.New()
	t.Cleanup(processes.Shutdown)
	target, source := filepath.Join(dir, "keep.zip"), filepath.Join(dir, "input.txt")
	require.NoError(t, os.WriteFile(target, []byte("original"), 0o600))
	require.NoError(t, os.WriteFile(source, []byte("input"), 0o600))
	require.Error(t, zipSources([]string{source}, 1, target, &processes))
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "original", string(data))
}

func TestOperationCompressionArchiveContents(t *testing.T) {
	dir := t.TempDir()
	source, target := filepath.Join(dir, "日本語.txt"), filepath.Join(dir, "output.zip")
	require.NoError(t, os.WriteFile(source, []byte("complete data"), 0o600))
	processes := processbar.New()
	t.Cleanup(processes.Shutdown)
	require.NoError(t, zipSources([]string{source}, 0, target, &processes))
	archive, err := zip.OpenReader(target)
	require.NoError(t, err)
	defer archive.Close()
	require.Len(t, archive.File, 1)
	file, err := archive.File[0].Open()
	require.NoError(t, err)
	defer file.Close()
	data, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, "complete data", string(data))
}

func TestOperationCancelledCopyAndMovePreserveSourceAndDestination(t *testing.T) {
	dir := t.TempDir()
	source, destination := filepath.Join(dir, "source"), filepath.Join(dir, "destination")
	require.NoError(t, os.WriteFile(source, []byte("source"), 0o600))
	require.NoError(t, os.WriteFile(destination, []byte("existing"), 0o600))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, copyFileContext(ctx, source, destination, 0o600), context.Canceled)
	require.ErrorIs(t, moveElementContext(ctx, source, destination), context.Canceled)
	require.Error(t, copyFileContext(context.Background(), source, destination, 0o600))
	data, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, "existing", string(data))
	require.FileExists(t, source)
}

type cancelAfterFirstRead struct {
	cancel context.CancelFunc
	read   bool
}

func (r *cancelAfterFirstRead) Read(buffer []byte) (int, error) {
	if r.read {
		return 0, errors.New("reader must not be called after cancellation")
	}
	r.read = true
	r.cancel()
	return copy(buffer, "first chunk"), nil
}

func TestOperationStreamCancellationStopsBetweenChunks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := &cancelAfterFirstRead{cancel: cancel}
	_, err := io.Copy(io.Discard, cancellationReader{ctx: ctx, reader: reader})
	require.ErrorIs(t, err, context.Canceled)
}

type failingArchiveWriter struct{ err error }

func (w failingArchiveWriter) Write(_ []byte) (int, error) { return 0, w.err }

func TestOperationZipFinalizationErrorIsReturned(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "small.txt")
	require.NoError(t, os.WriteFile(source, []byte("small buffered archive"), 0o600))
	processes := processbar.New()
	t.Cleanup(processes.Shutdown)
	p := processbar.NewProcess("close-error", source, processbar.OpCompress, 0)
	writeErr := errors.New("final ZIP write failed")
	err := writeZipContents(context.Background(), []string{source}, &processes, &p, failingArchiveWriter{err: writeErr})
	require.ErrorIs(t, err, writeErr)
}

func TestOperationSmallCopyAndGrowingSource(t *testing.T) {
	for _, snapshotSize := range []int64{1, 12} {
		dir := t.TempDir()
		source, destination := filepath.Join(dir, "source"), filepath.Join(dir, "destination")
		require.NoError(t, os.WriteFile(source, []byte("complete data"), 0o600))
		err := copyFileContextSized(
			context.Background(),
			source,
			destination,
			0o600,
			snapshotSize,
			make([]byte, copyBufferSize),
		)
		require.NoError(t, err)
		contents, err := os.ReadFile(destination)
		require.NoError(t, err)
		require.Equal(t, "complete data", string(contents), "a source growing past its snapshot must not be truncated")
		require.Error(
			t,
			copyFileContextSized(
				context.Background(),
				source,
				destination,
				0o600,
				snapshotSize,
				make([]byte, copyBufferSize),
			),
		)
	}
}

func TestOperationSmallCopyChecksCancellationBeforeCreate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	destination := filepath.Join(t.TempDir(), "not-created")
	require.ErrorIs(t, publishSmallCopy(ctx, destination, 0o600, []byte("buffered")), context.Canceled)
	require.NoFileExists(t, destination)
	require.ErrorIs(
		t,
		copyFileContextSized(
			context.Background(),
			"unused",
			destination,
			os.ModeNamedPipe,
			0,
			make([]byte, copyBufferSize),
		),
		errors.ErrUnsupported,
	)
}

type checkpointCancellation struct {
	context.Context

	cancel          context.CancelFunc
	checks, trigger int
}

func (c *checkpointCancellation) Err() error {
	c.checks++
	if c.checks == c.trigger {
		c.cancel()
	}
	return c.Context.Err()
}

func TestOperationCrossVolumeMoveAndCancellation(t *testing.T) {
	base := os.Getenv("XELIF_CROSS_VOLUME_DIR")
	if base == "" {
		t.Skip("requires an explicitly isolated second-volume fixture")
	}
	for _, cancelCopy := range []bool{false, true} {
		source := t.TempDir()
		//nolint:usetesting // t.TempDir always uses TEMP; this test needs the explicitly supplied second volume.
		destinationRoot, err := os.MkdirTemp(base, "operation-cross-volume-")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(destinationRoot) })
		if strings.EqualFold(filepath.VolumeName(source), filepath.VolumeName(destinationRoot)) {
			t.Skip("fixture must be on another volume")
		}
		contents := []byte("complete cross-volume file")
		require.NoError(t, os.WriteFile(filepath.Join(source, "one.txt"), contents, 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(source, "two.txt"), contents, 0o600))
		destination := filepath.Join(destinationRoot, "moved")
		ctx := context.Background()
		if cancelCopy {
			child, cancel := context.WithCancel(ctx)
			defer cancel()
			// Cancel after traversal has begun, without scheduling-dependent sleeps.
			ctx = &checkpointCancellation{Context: child, cancel: cancel, trigger: 5}
		}
		err = moveElementContext(ctx, source, destination)
		if cancelCopy {
			require.ErrorIs(t, err, context.Canceled)
			require.DirExists(t, source)
			require.FileExists(t, filepath.Join(source, "one.txt"))
			require.DirExists(t, destination)
		} else {
			require.NoError(t, err)
			require.NoDirExists(t, source)
			actual, err := os.ReadFile(filepath.Join(destination, "two.txt"))
			require.NoError(t, err)
			require.Equal(t, contents, actual)
		}
	}
}
