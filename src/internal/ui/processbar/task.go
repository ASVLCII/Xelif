package processbar

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
)

// TaskSpec is captured on the event loop before a worker starts. GroupKey is
// stable for an automatic task kind/panel, never for an explicit file mutation.
type TaskSpec struct {
	Kind, Source, Destination, GroupKey string
	Background, Service                 bool
	CancelReason                        string
}

// TaskResult carries outcome counts without teaching the registry about the
// application's individual command-message types.
type TaskResult interface {
	TaskProgress() (done, total int, partial string)
}

type taskControl struct {
	cancel context.CancelFunc
	ctx    context.Context
	owned  bool
}

type taskBus struct {
	mu             sync.Mutex
	pending        map[string]Process
	wake           chan struct{}
	done           chan struct{}
	stop           sync.Once
	lastFlush      time.Time
	controls       map[string]taskControl
	ordered        []Process
	footer         []Process
	dirty          bool
	revision       uint64
	workers        sync.WaitGroup
	lifetime       context.Context
	cancelLifetime context.CancelFunc
}

func newTaskBus() *taskBus {
	ctx, cancel := context.WithCancel(context.Background())
	return &taskBus{
		lifetime: ctx, cancelLifetime: cancel,
		pending: make(map[string]Process), wake: make(chan struct{}, 1), done: make(chan struct{}),
		controls: make(map[string]taskControl), dirty: true,
	}
}

// StartTask is an event-loop operation. Progress and completion from workers
// still pass through GetListenCmd; workers never mutate the displayed registry.
func (m *Model) StartTask(spec TaskSpec, ctx context.Context, cancel context.CancelFunc) Process {
	m.ensureInitialized()
	p := NewProcess(m.newUUIDForProcess(), spec.Source, OpTask, 0)
	p.Kind, p.Source, p.Destination = spec.Kind, spec.Source, spec.Destination
	p.Background, p.GroupKey, p.Service = spec.Background, spec.GroupKey, spec.Service
	p.CancelReason = spec.CancelReason
	if cancel == nil && p.CancelReason == "" {
		p.CancelReason = "This operation cannot be interrupted safely."
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m.bus.mu.Lock()
	m.bus.controls[p.ID] = taskControl{ctx: ctx, cancel: cancel}
	m.bus.mu.Unlock()
	_ = m.AddProcess(p)
	return p
}

func (m *Model) ensureInitialized() {
	if m.bus == nil {
		m.bus = newTaskBus()
	}
	if m.processesMu == nil {
		m.processesMu = &sync.RWMutex{}
	}
	if m.processes == nil {
		m.processes = make(map[string]Process)
	}
	if m.reqCnt == nil {
		m.reqCnt = &atomic.Int64{}
	}
	if m.msgChan == nil {
		m.msgChan = make(chan UpdateMsg, msgChannelSize)
	}
}

// Track preserves the original command result and the application's existing
// result dispatch. Completion of a failed command is never presented as success.
func (m *Model) Track(spec TaskSpec, ctx context.Context, cancel context.CancelFunc,
	cmd tea.Cmd, resultError func(tea.Msg) error,
) tea.Cmd {
	if cmd == nil {
		return nil
	}
	p := m.StartTask(spec, ctx, cancel)
	return m.Run(func() tea.Msg {
		msg := cmd()
		var err error
		if resultError != nil {
			err = resultError(msg)
		}
		if result, ok := msg.(TaskResult); ok {
			p.Done, p.Total, p.Partial = result.TaskProgress()
		}
		m.FinishTask(p, err)
		return msg
	})
}

// Run joins finite work to the model lifetime without creating another task.
func (m *Model) Run(cmd tea.Cmd) tea.Cmd {
	m.ensureInitialized()
	return func() tea.Msg {
		m.bus.mu.Lock()
		select {
		case <-m.bus.done:
			m.bus.mu.Unlock()
			return nil
		default:
			m.bus.workers.Add(1)
		}
		m.bus.mu.Unlock()
		defer m.bus.workers.Done()
		return cmd()
	}
}

// Call only after Shutdown, which prevents new workers from registering.
func (m *Model) WaitWorkers() {
	if m.bus != nil {
		m.bus.workers.Wait()
	}
}

func (m *Model) FinishTask(p Process, err error) {
	p.DoneTime = time.Now()
	p.State = Successful
	if err != nil {
		p.State = Failed
		p.ErrorMsg = err.Error()
		if errors.Is(err, context.Canceled) {
			p.State = Cancelled
		}
	}
	_ = m.SendUpdateProcessMsg(p, true)
}

// Cancel only requests cooperative cancellation. It does not invent a terminal
// result, undo completed work, or claim that an OS call has already stopped.
func (m *Model) Cancel(id string) bool {
	m.bus.mu.Lock()
	c := m.bus.controls[id]
	m.bus.mu.Unlock()
	if c.cancel == nil {
		return false
	}
	m.processesMu.Lock()
	p, exists := m.processes[id]
	if !exists || p.State != InOperation {
		m.processesMu.Unlock()
		return false
	}
	p.CancelRequested = true
	m.processes[id] = p
	m.invalidateOrder()
	m.processesMu.Unlock()
	c.cancel()
	return true
}

func (m *Model) ProcessContext(id string) context.Context {
	if m.bus == nil {
		return context.Background()
	}
	m.bus.mu.Lock()
	defer m.bus.mu.Unlock()
	if c, ok := m.bus.controls[id]; ok {
		return c.ctx
	}
	select {
	case <-m.bus.done:
		return m.bus.lifetime
	default:
	}
	return context.Background()
}

func (m *Model) CanCancel(id string) bool {
	m.bus.mu.Lock()
	defer m.bus.mu.Unlock()
	return m.bus.controls[id].cancel != nil
}

// Shutdown unblocks lifecycle producers/listeners before cancelling workers.
// It is safe to call repeatedly, including after a failed integration test.
func (m *Model) Shutdown() {
	if m.bus == nil {
		return
	}
	m.bus.mu.Lock()
	m.bus.stop.Do(func() { close(m.bus.done) })
	m.bus.cancelLifetime()
	controls := m.bus.controls
	m.bus.controls = make(map[string]taskControl)
	m.bus.pending = make(map[string]Process)
	m.bus.mu.Unlock()
	for _, c := range controls {
		if c.cancel != nil {
			c.cancel()
		}
	}
}

func (m *Model) ClearFinished() {
	m.processesMu.Lock()
	defer m.processesMu.Unlock()
	for id, p := range m.processes {
		if !p.DoneTime.IsZero() {
			delete(m.processes, id)
		}
	}
	m.cursor, m.renderIndex = 0, 0
	m.invalidateOrder()
}

func (m *Model) invalidateOrder() {
	if m.bus != nil {
		m.bus.mu.Lock()
		m.bus.dirty = true
		m.bus.revision++
		m.bus.mu.Unlock()
	}
}

func (m *Model) TaskRevision() uint64 {
	if m.bus == nil {
		return 0
	}
	m.bus.mu.Lock()
	defer m.bus.mu.Unlock()
	return m.bus.revision
}

// ObserveSync reports work that already runs on the event loop. It creates no
// extra commands, ticks or worker. Cancellation is unavailable during the call.
func (m *Model) ObserveSync(spec TaskSpec, run func() error) error {
	p := m.StartTask(spec, nil, nil)
	err := run()
	p.State, p.DoneTime = Successful, time.Now()
	if err != nil {
		p.State, p.ErrorMsg = Failed, err.Error()
	}
	_ = m.UpdateExistingProcess(p)
	return err
}

func (m *Model) finishControl(id string) {
	m.bus.mu.Lock()
	c := m.bus.controls[id]
	delete(m.bus.controls, id)
	delete(m.bus.pending, id)
	m.bus.mu.Unlock()
	if c.owned && c.cancel != nil {
		c.cancel()
	}
}

// TaskSnapshot returns a defensive copy for filters/details. The footer uses
// the cached immutable ordering directly and only renders visible rows.
func (m *Model) TaskSnapshot() []Process {
	return append([]Process(nil), m.getSortedProcesses()...)
}

// The caller owns processesMu. Concurrent requests retain separate identities.
func (m *Model) groupFinishedTask(id string) {
	p := m.processes[id]
	if !p.Background || p.GroupKey == "" {
		return
	}
	for otherID, old := range m.processes {
		if otherID != id && old.Background && old.GroupKey == p.GroupKey && !old.DoneTime.IsZero() {
			if old.Started.After(p.Started) {
				old.Runs += p.Runs
				m.processes[otherID] = old
				delete(m.processes, id)
				return
			}
			p.Runs += old.Runs
			delete(m.processes, otherID)
		}
	}
	m.processes[id] = p
}

func (p *Process) StatusText() string {
	switch {
	case p.State == Successful:
		return "Succeeded"
	case p.State == Failed:
		return "Failed"
	case p.State == Cancelled:
		return "Cancelled"
	case p.CancelRequested:
		return "Cancel requested"
	case p.Service:
		return "Listening"
	default:
		return "Running"
	}
}

func (p *Process) TaskName() string {
	if p.Kind != "" {
		return p.Kind
	}
	return p.Operation.GetVerb()
}
