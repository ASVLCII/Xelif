package processbar

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

func TestTaskCancellationWaitsForCompletion(t *testing.T) {
	m := New()
	t.Cleanup(m.Shutdown)
	ctx, cancel := context.WithCancel(context.Background())
	p := m.StartTask(TaskSpec{Kind: "copy"}, ctx, cancel)
	require.True(t, m.Cancel(p.ID))
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	pending, ok := m.GetByID(p.ID)
	require.True(t, ok)
	require.Equal(t, InOperation, pending.State)
	require.Equal(t, "Cancel requested", pending.StatusText())
	m.FinishTask(p, ctx.Err())
	_, err := m.GetListenCmd()().Apply(&m)
	require.NoError(t, err)
	finished, _ := m.GetByID(p.ID)
	require.Equal(t, Cancelled, finished.State)
	require.False(t, finished.DoneTime.IsZero())
	require.False(t, m.CanCancel(p.ID))
	require.False(t, m.Cancel(p.ID))
	// Neither stale progress nor duplicate completion may regress this outcome.
	require.NoError(t, m.UpdateExistingProcess(p))
	p.State, p.DoneTime = Successful, time.Now()
	require.NoError(t, m.UpdateExistingProcess(p))
	finished, _ = m.GetByID(p.ID)
	require.Equal(t, Cancelled, finished.State)
}

func TestTaskProgressBurstKeepsLatestWithoutLifecycleFlood(t *testing.T) {
	m := New()
	t.Cleanup(m.Shutdown)
	p := NewProcess("burst", "file", OpCopy, 100_000)
	require.NoError(t, m.AddProcess(p))
	for i := range 100_000 {
		p.Done = i
		m.TrySendingUpdateProcessMsg(p)
	}
	require.Empty(t, m.msgChan)
	require.Len(t, m.bus.pending, 1)
	_, err := m.GetListenCmd()().Apply(&m)
	require.NoError(t, err)
	stored, _ := m.GetByID(p.ID)
	require.Equal(t, 99_999, stored.Done)
	require.Empty(t, m.bus.pending)
}

func TestTaskCompletionBypassesProgressDelay(t *testing.T) {
	m := New()
	t.Cleanup(m.Shutdown)
	p := m.StartTask(TaskSpec{Kind: "delayed progress"}, nil, nil)
	m.bus.lastFlush = time.Now().Add(time.Hour)
	m.TrySendingUpdateProcessMsg(p)
	result := make(chan UpdateMsg, 1)
	go func() { result <- m.GetListenCmd()() }()
	m.FinishTask(p, nil)
	select {
	case msg := <-result:
		_, err := msg.Apply(&m)
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("completion was trapped behind progress delay")
	}
	stored, _ := m.GetByID(p.ID)
	require.Equal(t, Successful, stored.State)
}

func TestTaskShutdownReleasesBlockedLifecycleProducers(t *testing.T) {
	m := New()
	for range msgChannelSize {
		require.NoError(t, m.trySendMsgToChannel(stopListeningMsg{}))
	}
	finished := make(chan struct{})
	go func() { _ = m.SendUpdateProcessMsg(NewProcess("blocked", "", OpCopy, 1), true); close(finished) }()
	m.Shutdown()
	m.Shutdown()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not release producer")
	}
	_, err := m.GetListenCmd()().Apply(&m)
	require.NoError(t, err)
}

func TestTaskWorkerCannotMutateDisplayedRegistry(t *testing.T) {
	m := New()
	t.Cleanup(m.Shutdown)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := m.Track(TaskSpec{Kind: "archive"}, ctx, cancel, func() tea.Msg { return "result" }, nil)
	require.Equal(t, "result", cmd())
	rows := m.TaskSnapshot()
	require.Len(t, rows, 1)
	require.Equal(t, InOperation, rows[0].State)
	_, err := m.GetListenCmd()().Apply(&m)
	require.NoError(t, err)
	require.NoError(t, ctx.Err(), "external contexts must remain valid for original result dispatch")
	require.Equal(t, Successful, m.TaskSnapshot()[0].State)
}

func TestTaskHistoryBoundAndClearPreserveActiveServices(t *testing.T) {
	m := New()
	t.Cleanup(m.Shutdown)
	service := m.StartTask(TaskSpec{Kind: "watcher", Background: true, Service: true}, nil, nil)
	for i := range 150 {
		p := m.StartTask(TaskSpec{Kind: strconv.Itoa(i), Background: i%2 == 0}, nil, nil)
		p.State, p.DoneTime = Successful, time.Now()
		require.NoError(t, m.UpdateExistingProcess(p))
	}
	require.Len(t, m.TaskSnapshot(), 101)
	m.ClearFinished()
	rows := m.TaskSnapshot()
	require.Len(t, rows, 1)
	require.Equal(t, service.ID, rows[0].ID)
	require.False(t, m.HasRunningProcesses(), "background services must not prevent normal quitting")
	require.False(t, m.Cancel(service.ID))
}

func TestTaskBackgroundGroupKeepsNewestRequest(t *testing.T) {
	m := New()
	t.Cleanup(m.Shutdown)
	spec := TaskSpec{Kind: "preview", Background: true, GroupKey: "preview:0"}
	old := m.StartTask(spec, nil, nil)
	newer := m.StartTask(spec, nil, nil)
	old.Started, newer.Started = time.Unix(1, 0), time.Unix(2, 0)
	newer.State, newer.DoneTime = Successful, time.Now()
	require.NoError(t, m.UpdateExistingProcess(newer))
	old.State, old.DoneTime = Cancelled, time.Now()
	require.NoError(t, m.UpdateExistingProcess(old))
	rows := m.TaskSnapshot()
	require.Len(t, rows, 1)
	require.Equal(t, newer.ID, rows[0].ID)
	require.Equal(t, uint64(2), rows[0].Runs)
}

func TestTaskConcurrentProgressAndShutdown(t *testing.T) {
	m := New()
	var workers sync.WaitGroup
	for i := range 8 {
		workers.Go(func() {
			p := NewProcess(strconv.Itoa(i), "file", OpCopy, 1000)
			for j := range 1000 {
				p.Done = j
				m.TrySendingUpdateProcessMsg(p)
			}
		})
	}
	m.Shutdown()
	workers.Wait()
	require.Empty(t, m.bus.pending)
}

func TestTaskShutdownJoinsWorkersAndRejectsLateStarts(t *testing.T) {
	m := New()
	entered, release, joined := make(chan struct{}), make(chan struct{}), make(chan struct{})
	cmd := m.Run(func() tea.Msg { close(entered); <-release; return "finished" })
	go cmd()
	<-entered
	m.Shutdown()
	go func() { m.WaitWorkers(); close(joined) }()
	select {
	case <-joined:
		t.Fatal("worker was not joined")
	default:
	}
	close(release)
	select {
	case <-joined:
	case <-time.After(time.Second):
		t.Fatal("worker did not finish shutdown")
	}
	late := m.Run(func() tea.Msg { t.Error("worker started after shutdown"); return nil })
	require.Nil(t, late())
	require.ErrorIs(t, m.ProcessContext("released").Err(), context.Canceled)
}
