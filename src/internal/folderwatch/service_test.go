package folderwatch

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestSyncAddsRemovesAndIsIdempotent(t *testing.T) {
	t.Parallel()

	w := newFakeWatcher()
	s := newService(context.Background(), w, Options{CoalesceWindow: time.Millisecond, MaxBatchSize: 8})
	t.Cleanup(func() { _ = s.Close() })

	a := filepath.Clean(t.TempDir())
	b := filepath.Clean(t.TempDir())

	if err := s.Sync([]string{b, a, a}); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	if err := s.Sync([]string{a, b}); err != nil {
		t.Fatalf("idempotent Sync: %v", err)
	}
	if got, want := w.addedPaths(), []string{a, b}; !slices.Equal(got, want) {
		t.Fatalf("added paths = %q, want %q", got, want)
	}
	if got := w.removedPaths(); len(got) != 0 {
		t.Fatalf("unexpected removed paths: %q", got)
	}

	if err := s.Sync([]string{b}); err != nil {
		t.Fatalf("remove Sync: %v", err)
	}
	if got, want := w.removedPaths(), []string{a}; !slices.Equal(got, want) {
		t.Fatalf("removed paths = %q, want %q", got, want)
	}

	// Removing an already absent path is harmless and makes no backend call.
	if err := s.Sync([]string{b}); err != nil {
		t.Fatalf("second remove Sync: %v", err)
	}
	if got, want := w.removedPaths(), []string{a}; !slices.Equal(got, want) {
		t.Fatalf("removed paths after idempotent Sync = %q, want %q", got, want)
	}
}

func TestSyncReportsRegistrationErrorsAndKeepsValidPaths(t *testing.T) {
	t.Parallel()

	w := newFakeWatcher()
	missing := filepath.Clean(filepath.Join(t.TempDir(), "missing"))
	valid := filepath.Clean(t.TempDir())
	w.addErrors[missing] = fs.ErrNotExist
	s := newService(context.Background(), w, testOptions())
	t.Cleanup(func() { _ = s.Close() })

	err := s.Sync([]string{missing, missing, valid})
	if err == nil {
		t.Fatal("Sync error = nil, want missing-path registration error")
	}
	var pathErr *PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("Sync error %T = %v, want *PathError", err, err)
	}
	if pathErr.Op != "add" || pathErr.Path != missing || !errors.Is(pathErr, fs.ErrNotExist) {
		t.Fatalf("PathError = %#v, want add error for %q wrapping fs.ErrNotExist", pathErr, missing)
	}
	if got, want := w.addedPaths(), []string{missing, valid}; !slices.Equal(got, want) {
		t.Fatalf("add attempts = %q, want %q", got, want)
	}

	w.events <- fsnotify.Event{Name: filepath.Join(valid, "created.txt"), Op: fsnotify.Create}
	event := receiveEvent(t, s.Events())
	if got, want := event.Paths, []string{valid}; !slices.Equal(got, want) {
		t.Fatalf("event paths = %q, want %q", got, want)
	}
}

func TestBurstIsCoalescedIntoSortedBoundedEvents(t *testing.T) {
	t.Parallel()

	w := newFakeWatcher()
	s := newService(context.Background(), w, Options{CoalesceWindow: 40 * time.Millisecond, MaxBatchSize: 2})
	t.Cleanup(func() { _ = s.Close() })

	dirs := []string{filepath.Clean(t.TempDir()), filepath.Clean(t.TempDir()), filepath.Clean(t.TempDir())}
	slices.Sort(dirs)
	if err := s.Sync([]string{dirs[2], dirs[0], dirs[1]}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// Duplicate backend events for one directory collapse to one invalidation.
	w.events <- fsnotify.Event{Name: filepath.Join(dirs[2], "z"), Op: fsnotify.Write}
	w.events <- fsnotify.Event{Name: filepath.Join(dirs[0], "a"), Op: fsnotify.Create}
	w.events <- fsnotify.Event{Name: filepath.Join(dirs[2], "z"), Op: fsnotify.Write}
	w.events <- fsnotify.Event{Name: filepath.Join(dirs[1], "m"), Op: fsnotify.Remove}

	first := receiveEvent(t, s.Events())
	second := receiveEvent(t, s.Events())
	if got, want := first.Paths, dirs[:2]; !slices.Equal(got, want) {
		t.Fatalf("first event paths = %q, want %q", got, want)
	}
	if got, want := second.Paths, dirs[2:]; !slices.Equal(got, want) {
		t.Fatalf("second event paths = %q, want %q", got, want)
	}
}

func TestRuntimeWatcherErrorsAreExposed(t *testing.T) {
	t.Parallel()

	w := newFakeWatcher()
	s := newService(context.Background(), w, testOptions())
	t.Cleanup(func() { _ = s.Close() })

	want := errors.New("backend overflow")
	w.errs <- want
	select {
	case got := <-s.Errors():
		if !errors.Is(got, want) {
			t.Fatalf("runtime error = %v, want %v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for runtime watcher error")
	}
}

func TestRemovedPathDoesNotEmitEvent(t *testing.T) {
	t.Parallel()

	w := newFakeWatcher()
	s := newService(context.Background(), w, Options{CoalesceWindow: 10 * time.Millisecond, MaxBatchSize: 8})
	t.Cleanup(func() { _ = s.Close() })
	dir := filepath.Clean(t.TempDir())

	if err := s.Sync([]string{dir}); err != nil {
		t.Fatalf("add Sync: %v", err)
	}
	// Let an event reach the coalescer without receiving it. Sync must discard
	// that queued invalidation as part of removing the directory.
	w.events <- fsnotify.Event{Name: filepath.Join(dir, "queued.txt"), Op: fsnotify.Write}
	time.Sleep(30 * time.Millisecond)
	if err := s.Sync(nil); err != nil {
		t.Fatalf("remove Sync: %v", err)
	}
	w.events <- fsnotify.Event{Name: filepath.Join(dir, "late.txt"), Op: fsnotify.Create}

	select {
	case event := <-s.Events():
		t.Fatalf("unexpected event after path removal: %#v", event)
	case <-time.After(80 * time.Millisecond):
	}
}

func TestCancelAndCloseStopService(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	w := newFakeWatcher()
	s := newService(ctx, w, testOptions())
	cancel()

	select {
	case _, ok := <-s.Events():
		if ok {
			t.Fatal("Events remained open after context cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("Events was not closed after context cancellation")
	}
	if err := s.Sync([]string{t.TempDir()}); !errors.Is(err, ErrClosed) {
		t.Fatalf("Sync after cancel = %v, want ErrClosed", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if got := w.closeCount(); got != 1 {
		t.Fatalf("backend Close calls = %d, want 1", got)
	}
}

func TestRealWatcherCreateRenameDelete(t *testing.T) {
	dir := filepath.Clean(t.TempDir())
	s, err := New(context.Background(), Options{CoalesceWindow: 20 * time.Millisecond, MaxBatchSize: 8})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Sync([]string{dir}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	oldName := filepath.Join(dir, "old.txt")
	newName := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(oldName, []byte("x"), 0o600); err != nil {
		t.Fatalf("create file: %v", err)
	}
	assertDirectoryEvent(t, s, dir, "create")
	if err := os.Rename(oldName, newName); err != nil {
		t.Fatalf("rename file: %v", err)
	}
	assertDirectoryEvent(t, s, dir, "rename")
	if err := os.Remove(newName); err != nil {
		t.Fatalf("delete file: %v", err)
	}
	assertDirectoryEvent(t, s, dir, "delete")
}

func assertDirectoryEvent(t *testing.T, s Service, dir, operation string) {
	t.Helper()
	select {
	case event, ok := <-s.Events():
		if !ok {
			t.Fatalf("Events closed while waiting for %s", operation)
		}
		if got, want := event.Paths, []string{dir}; !slices.Equal(got, want) {
			t.Fatalf("%s event paths = %q, want %q", operation, got, want)
		}
	case err, ok := <-s.Errors():
		if !ok {
			t.Fatalf("Errors closed while waiting for %s", operation)
		}
		t.Fatalf("watcher error while waiting for %s: %v", operation, err)
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s event", operation)
	}
}

func receiveEvent(t *testing.T, events <-chan Event) Event {
	t.Helper()
	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("Events closed unexpectedly")
		}
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
		return Event{}
	}
}

func testOptions() Options {
	return Options{CoalesceWindow: time.Millisecond, MaxBatchSize: 8}
}

type fakeWatcher struct {
	events chan fsnotify.Event
	errs   chan error

	mu         sync.Mutex
	adds       []string
	removes    []string
	closes     int
	addErrors  map[string]error
	removeErrs map[string]error
}

func newFakeWatcher() *fakeWatcher {
	return &fakeWatcher{
		events:     make(chan fsnotify.Event, 32),
		errs:       make(chan error, 8),
		addErrors:  make(map[string]error),
		removeErrs: make(map[string]error),
	}
}

func (w *fakeWatcher) Add(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.adds = append(w.adds, path)
	return w.addErrors[path]
}

func (w *fakeWatcher) Remove(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.removes = append(w.removes, path)
	return w.removeErrs[path]
}

func (w *fakeWatcher) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closes++
	return nil
}

func (w *fakeWatcher) Events() <-chan fsnotify.Event { return w.events }
func (w *fakeWatcher) Errors() <-chan error          { return w.errs }

func (w *fakeWatcher) addedPaths() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.adds)
}

func (w *fakeWatcher) removedPaths() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.removes)
}

func (w *fakeWatcher) closeCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closes
}
