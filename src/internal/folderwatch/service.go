// Package folderwatch reports changes to a synchronized set of local directories.
package folderwatch

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	defaultCoalesceWindow = 40 * time.Millisecond
	defaultMaxBatchSize   = 64
)

// ErrClosed is returned when an operation is attempted after shutdown starts.
var ErrClosed = errors.New("folderwatch: service closed")

// Event identifies watched directories whose contents may have changed. Paths
// are absolute, unique, and sorted. Callers must not mutate Paths.
type Event struct {
	Paths []string
}

// Options controls event coalescing. The window is measured from the first
// change in a burst and each emitted event contains at most MaxBatchSize paths.
// Zero values select bounded defaults.
type Options struct {
	CoalesceWindow time.Duration
	MaxBatchSize   int
}

// Service watches exactly the successfully registered paths passed to its most
// recent Sync call.
type Service interface {
	Sync(paths []string) error
	Events() <-chan Event
	Errors() <-chan error
	Close() error
}

// PathError describes a path registration or removal failure.
type PathError struct {
	Op   string
	Path string
	Err  error
}

func (e *PathError) Error() string {
	return fmt.Sprintf("folderwatch: %s %q: %v", e.Op, e.Path, e.Err)
}

func (e *PathError) Unwrap() error { return e.Err }

// New starts a filesystem-backed service. At most one Options value may be
// supplied; omitting it uses the defaults.
func New(ctx context.Context, options ...Options) (Service, error) {
	if ctx == nil {
		return nil, errors.New("folderwatch: nil context")
	}
	opts, err := resolveOptions(options)
	if err != nil {
		return nil, err
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("folderwatch: create watcher: %w", err)
	}
	return newService(ctx, fsnotifyBackend{Watcher: w}, opts), nil
}

type backend interface {
	Add(string) error
	Remove(string) error
	Close() error
	Events() <-chan fsnotify.Event
	Errors() <-chan error
}

type fsnotifyBackend struct {
	*fsnotify.Watcher
}

func (w fsnotifyBackend) Events() <-chan fsnotify.Event { return w.Watcher.Events }
func (w fsnotifyBackend) Errors() <-chan error          { return w.Watcher.Errors }

type syncRequest struct {
	paths []string
	done  chan error
}

type service struct {
	ctx      context.Context
	cancel   context.CancelFunc
	watcher  backend
	options  Options
	syncReqs chan syncRequest
	events   chan Event
	errors   chan error
	done     chan struct{}

	closeMu  sync.Mutex
	closeErr error
}

func newService(parent context.Context, watcher backend, options Options) *service {
	options, _ = resolveOptions([]Options{options})
	ctx, cancel := context.WithCancel(parent)
	s := &service{
		ctx:      ctx,
		cancel:   cancel,
		watcher:  watcher,
		options:  options,
		syncReqs: make(chan syncRequest),
		events:   make(chan Event),
		errors:   make(chan error),
		done:     make(chan struct{}),
	}
	go s.run()
	return s
}

func resolveOptions(values []Options) (Options, error) {
	if len(values) > 1 {
		return Options{}, errors.New("folderwatch: multiple Options values")
	}
	var options Options
	if len(values) == 1 {
		options = values[0]
	}
	if options.CoalesceWindow < 0 {
		return Options{}, errors.New("folderwatch: CoalesceWindow must not be negative")
	}
	if options.MaxBatchSize < 0 {
		return Options{}, errors.New("folderwatch: MaxBatchSize must not be negative")
	}
	if options.CoalesceWindow == 0 {
		options.CoalesceWindow = defaultCoalesceWindow
	}
	if options.MaxBatchSize == 0 {
		options.MaxBatchSize = defaultMaxBatchSize
	}
	return options, nil
}

func (s *service) Sync(paths []string) error {
	request := syncRequest{paths: slices.Clone(paths), done: make(chan error, 1)}
	select {
	case s.syncReqs <- request:
	case <-s.ctx.Done():
		return ErrClosed
	}
	select {
	case err := <-request.done:
		return err
	case <-s.ctx.Done():
		return ErrClosed
	}
}

func (s *service) Events() <-chan Event { return s.events }
func (s *service) Errors() <-chan error { return s.errors }

func (s *service) Close() error {
	s.cancel()
	<-s.done
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	return s.closeErr
}

func (s *service) run() {
	defer close(s.done)
	defer close(s.errors)
	defer close(s.events)
	defer s.closeWatcher()

	state := newWatchLoopState(s.watcher)

	for {
		eventOut, nextEvent := state.nextEvent(s.events, s.options.MaxBatchSize)
		errorOut, errorIn := state.errorChannels(s.errors)

		select {
		case <-s.ctx.Done():
			state.stopTimer()
			return
		case request := <-s.syncReqs:
			request.done <- s.applySync(state.active, state.pending, state.ready, request.paths)
		case eventOut <- nextEvent:
			state.markEventSent(nextEvent)
		case errorOut <- state.pendingError:
			state.pendingError = nil
		case event, ok := <-state.watchEvents:
			state.handleWatchEvent(event, ok, s.options.CoalesceWindow)
		case <-state.timerC:
			state.flushPending()
		case err, ok := <-errorIn:
			state.handleWatchError(err, ok)
		}
	}
}

func (s *service) closeWatcher() {
	err := s.watcher.Close()
	s.closeMu.Lock()
	s.closeErr = err
	s.closeMu.Unlock()
}

type watchLoopState struct {
	active       map[string]string
	pending      map[string]struct{}
	ready        map[string]struct{}
	watchEvents  <-chan fsnotify.Event
	watchErrors  <-chan error
	pendingError error
	timer        *time.Timer
	timerC       <-chan time.Time
}

func newWatchLoopState(watcher backend) *watchLoopState {
	return &watchLoopState{
		active:      make(map[string]string),
		pending:     make(map[string]struct{}),
		ready:       make(map[string]struct{}),
		watchEvents: watcher.Events(),
		watchErrors: watcher.Errors(),
	}
}

func (s *watchLoopState) nextEvent(output chan Event, maxBatchSize int) (chan Event, Event) {
	if len(s.ready) == 0 {
		return nil, Event{}
	}
	paths := mapKeys(s.ready)
	if len(paths) > maxBatchSize {
		paths = paths[:maxBatchSize]
	}
	return output, Event{Paths: paths}
}

func (s *watchLoopState) errorChannels(output chan error) (chan error, <-chan error) {
	if s.pendingError != nil {
		return output, nil
	}
	return nil, s.watchErrors
}

func (s *watchLoopState) markEventSent(event Event) {
	for _, path := range event.Paths {
		delete(s.ready, path)
	}
}

func (s *watchLoopState) handleWatchEvent(event fsnotify.Event, ok bool, coalesceWindow time.Duration) {
	if !ok {
		s.watchEvents = nil
		return
	}
	for _, path := range affectedPaths(s.active, event.Name) {
		s.pending[path] = struct{}{}
	}
	if len(s.pending) > 0 && s.timerC == nil {
		s.startTimer(coalesceWindow)
	}
}

func (s *watchLoopState) handleWatchError(err error, ok bool) {
	if !ok {
		s.watchErrors = nil
		return
	}
	if err != nil && s.pendingError == nil {
		s.pendingError = fmt.Errorf("folderwatch: watcher: %w", err)
	}
}

func (s *watchLoopState) startTimer(coalesceWindow time.Duration) {
	if s.timer == nil {
		s.timer = time.NewTimer(coalesceWindow)
	} else {
		s.timer.Reset(coalesceWindow)
	}
	s.timerC = s.timer.C
}

func (s *watchLoopState) stopTimer() {
	if s.timer != nil && !s.timer.Stop() {
		select {
		case <-s.timer.C:
		default:
		}
	}
	s.timerC = nil
}

func (s *watchLoopState) flushPending() {
	for path := range s.pending {
		s.ready[path] = struct{}{}
		delete(s.pending, path)
	}
	s.stopTimer()
}

func (s *service) applySync(active map[string]string, pending, ready map[string]struct{}, paths []string) error {
	desired := make(map[string]string, len(paths))
	var errs []error
	for _, path := range paths {
		if path == "" {
			continue
		}
		absolute, err := filepath.Abs(filepath.Clean(path))
		if err != nil {
			errs = append(errs, &PathError{Op: "normalize", Path: path, Err: err})
			continue
		}
		desired[pathKey(absolute)] = absolute
	}

	var removals []string
	for key, path := range active {
		if _, keep := desired[key]; !keep {
			removals = append(removals, path)
		}
	}
	slices.Sort(removals)
	for _, path := range removals {
		key := pathKey(path)
		delete(active, key)
		delete(pending, path)
		delete(ready, path)
		if err := s.watcher.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, &PathError{Op: "remove", Path: path, Err: err})
		}
	}

	var additions []string
	for key, path := range desired {
		if _, exists := active[key]; !exists {
			additions = append(additions, path)
		}
	}
	slices.Sort(additions)
	for _, path := range additions {
		if err := s.watcher.Add(path); err != nil {
			errs = append(errs, &PathError{Op: "add", Path: path, Err: err})
			continue
		}
		active[pathKey(path)] = path
	}
	return errors.Join(errs...)
}

func affectedPaths(active map[string]string, changed string) []string {
	if changed == "" {
		return nil
	}
	changed, err := filepath.Abs(filepath.Clean(changed))
	if err != nil {
		return nil
	}
	parent := filepath.Dir(changed)
	var paths []string
	for _, watched := range active {
		if samePath(changed, watched) || samePath(parent, watched) {
			paths = append(paths, watched)
		}
	}
	slices.Sort(paths)
	return paths
}

func samePath(a, b string) bool { return pathKey(a) == pathKey(b) }

func pathKey(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func mapKeys(set map[string]struct{}) []string {
	paths := make([]string, 0, len(set))
	for path := range set {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}
