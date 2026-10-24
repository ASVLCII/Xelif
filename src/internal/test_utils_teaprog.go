package internal

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"charm.land/bubbles/v2/cursor"
	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/ui/processbar"
	"github.com/yorukot/superfile/src/pkg/utils"
)

type IgnorerWriter struct{}

func (w IgnorerWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

type TeaProg struct {
	m         *model
	prog      *tea.Program
	t         *testing.T
	done      chan struct{}
	err       error
	owner     *testProgramModel
	closeOnce sync.Once
}

// The event loop owns the model. A barrier lends that ownership to the
// caller until release, including when the caller exits via testing.Fatal.
type testModelBarrier struct {
	entered chan struct{}
	release chan struct{}
}

type testCommandResult struct {
	msg     tea.Msg
	initial bool
}
type testCommandApplied struct{}

type testProgramModel struct {
	*model

	pending     int
	changed     chan struct{}
	closing     bool
	initialized chan struct{}
}

func (m *testProgramModel) changedPending(delta int) {
	m.pending += delta
	close(m.changed)
	m.changed = make(chan struct{})
}

func (m *testProgramModel) track(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	m.changedPending(1)
	return func() tea.Msg { return testCommandResult{msg: cmd()} }
}

func (m *testProgramModel) Init() tea.Cmd {
	cmd := m.track(m.model.Init())
	return func() tea.Msg {
		result, ok := cmd().(testCommandResult)
		if !ok {
			panic("tracked initialization returned an unexpected message")
		}
		result.initial = true
		return result
	}
}

func (m *testProgramModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if barrier, ok := msg.(testModelBarrier); ok {
		close(barrier.entered)
		<-barrier.release
		return m, nil
	}
	if _, ok := msg.(testCommandApplied); ok {
		m.changedPending(-1)
		return m, nil
	}
	if result, ok := msg.(testCommandResult); ok {
		if result.initial {
			defer close(m.initialized)
		}
		// Keep the parent pending until any children and subsequent commands are
		// registered; shutdown must never observe a transient zero between them.
		completedHere := true
		defer func() {
			if completedHere {
				m.changedPending(-1)
			}
		}()
		msg = result.msg
		if batch, ok := msg.(tea.BatchMsg); ok {
			children := make([]tea.Cmd, 0, len(batch))
			for _, cmd := range batch {
				children = append(children, m.track(cmd))
			}
			return m, tea.Batch(children...)
		}
		if msg == nil {
			return m, nil
		}
		// Bubble Tea handles several private message types before calling Update.
		// Replay those through its dispatcher, then acknowledge on the next turn.
		if reflect.TypeOf(msg).PkgPath() == reflect.TypeOf(tea.QuitMsg{}).PkgPath() {
			completedHere = false
			return m, tea.Sequence(
				func() tea.Msg { return msg },
				func() tea.Msg { return testCommandApplied{} },
			)
		}
	}
	return m.updateModel(msg)
}

func (m *testProgramModel) updateModel(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.closing {
		switch msg.(type) {
		case refreshTickMsg, cursor.BlinkMsg:
			return m, nil
		}
	}
	next, cmd := m.model.Update(msg)
	nextModel, ok := next.(*model)
	if !ok {
		panic("test update returned an unexpected model")
	}
	m.model = nextModel
	return m, m.track(cmd)
}

func NewTestTeaProgWithEventLoop(t *testing.T, m *model, options ...tea.ProgramOption) *TeaProg {
	defaults := []tea.ProgramOption{
		tea.WithInput(nil), tea.WithOutput(IgnorerWriter{}),
		tea.WithWindowSize(DefaultTestModelWidth, DefaultTestModelHeight),
	}
	p := &TeaProg{m: m, t: t, done: make(chan struct{})}
	p.owner = &testProgramModel{model: m, changed: make(chan struct{}), initialized: make(chan struct{})}
	p.prog = tea.NewProgram(p.owner, append(defaults, options...)...)
	t.Cleanup(p.Close)
	p.StartEventLoop()
	return p
}

type testFileOperationSignal struct {
	done  chan struct{}
	once  sync.Once
	state processbar.ProcessState
}

// Observe one compression and one extraction without blocking the event loop.
func observeTestFileOperations() (tea.ProgramOption, *testFileOperationSignal, *testFileOperationSignal) {
	compressed := &testFileOperationSignal{done: make(chan struct{})}
	extracted := &testFileOperationSignal{done: make(chan struct{})}
	filter := tea.WithFilter(func(_ tea.Model, msg tea.Msg) tea.Msg {
		observed := msg
		if result, ok := observed.(testCommandResult); ok {
			observed = result.msg
		}
		switch operation := observed.(type) {
		case CompressOperationMsg:
			compressed.complete(operation.state)
		case ExtractOperationMsg:
			extracted.complete(operation.state)
		}
		return msg
	})
	return filter, compressed, extracted
}

func (signal *testFileOperationSignal) complete(state processbar.ProcessState) {
	signal.once.Do(func() {
		signal.state = state
		close(signal.done)
	})
}

func (signal *testFileOperationSignal) wait(t *testing.T, operation string) bool {
	t.Helper()
	timer := time.NewTimer(DefaultTestTimeout)
	defer timer.Stop()
	select {
	case <-signal.done:
		return true
	case <-timer.C:
		t.Errorf("%s command completion was not observed", operation)
		return false
	}
}

func NewTestTeaProgWithEventLoopWithWinSize(t *testing.T, m *model, width int, height int) *TeaProg {
	return NewTestTeaProgWithEventLoop(t, m, tea.WithWindowSize(width, height))
}

func (p *TeaProg) StartEventLoop() {
	go func() {
		defer close(p.done)
		_, p.err = p.prog.Run()
	}()
	// Init returns a batch asynchronously. Its children (including the permanent
	// processbar listener) must be registered before shutdown can recognize idle.
	timer := time.NewTimer(DefaultTestTimeout)
	defer timer.Stop()
	select {
	case <-p.owner.initialized:
	case <-p.done:
		p.t.Fatalf("test event loop failed during initialization: %v", p.err)
	case <-timer.C:
		p.prog.Kill()
		p.t.Fatal("timed out initializing test event loop")
	}
	// A received message alone does not acknowledge a completed Update.
	p.WithModel(func(*model) {})
}

// WithModel executes on the caller goroutine while Update and View are parked.
// Do not retain mutable model pointers or call Send/WithModel from the callback.
func (p *TeaProg) WithModel(callback func(*model)) {
	p.t.Helper()
	barrier := testModelBarrier{entered: make(chan struct{}), release: make(chan struct{})}
	defer close(barrier.release)
	p.send(barrier)
	timer := time.NewTimer(DefaultTestTimeout)
	defer timer.Stop()
	select {
	case <-barrier.entered:
		callback(p.m)
	case <-p.done:
		p.t.Fatalf("test event loop stopped before model access: %v", p.err)
	case <-timer.C:
		p.t.Fatal("timed out waiting for model ownership")
	}
}

func (p *TeaProg) Check(predicate func(*model) bool) bool {
	var result bool
	p.WithModel(func(m *model) { result = predicate(m) })
	return result
}

// send bounds the unbuffered receive itself, not merely the later barrier.
func (p *TeaProg) send(msg tea.Msg) {
	p.t.Helper()
	sent := make(chan struct{})
	go func() { defer close(sent); p.prog.Send(msg) }()
	timer := time.NewTimer(DefaultTestTimeout)
	defer timer.Stop()
	select {
	case <-sent:
	case <-timer.C:
		p.prog.Kill()
		<-sent
		p.t.Fatal("timed out sending to test event loop")
	}
}

func (p *TeaProg) Send(msgs ...tea.Msg) {
	for _, msg := range msgs {
		p.send(msg)
		p.WithModel(func(*model) {})
	}
}

func (p *TeaProg) SendKey(key string) {
	p.Send(utils.TeaRuneKeyMsg(key))
}

// waitPending observes command completion after its result has been applied.
func (p *TeaProg) waitPending(maximum int) bool {
	const commandDrainTimeout = 10 * DefaultTestTimeout
	timer := time.NewTimer(commandDrainTimeout)
	defer timer.Stop()
	for {
		var pending int
		var changed <-chan struct{}
		p.WithModel(func(*model) { pending, changed = p.owner.pending, p.owner.changed })
		if pending <= maximum {
			return true
		}
		select {
		case <-changed:
		case <-p.done:
			p.t.Errorf("event loop stopped with %d pending commands", pending)
			return false
		case <-timer.C:
			p.t.Errorf("timed out draining %d test commands", pending)
			return false
		}
	}
}

func (p *TeaProg) Close() {
	p.closeOnce.Do(func() {
		select {
		case <-p.done:
		default:
			p.WithModel(func(m *model) {
				p.owner.closing = true
				if m.folderWatcher != nil {
					_ = m.folderWatcher.Close()
				}
			})
			// A live model always has one processbar channel listener. Keep it
			// consuming progress until all finite work and its results are finished.
			if p.waitPending(1) {
				p.m.processBarModel.SendStopListeningMsgBlocking()
				p.waitPending(0)
			}
			p.prog.Kill()
			<-p.done
		}
		if p.err != nil && !errors.Is(p.err, tea.ErrProgramKilled) {
			p.t.Errorf("test event loop failed: %v", p.err)
		}
	})
}
