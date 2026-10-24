package processbar

import (
	"context"
	"time"
)

const progressInterval = 100 * time.Millisecond

// Only used in tests, to have processbar used in a standalone way without model
func (m *Model) ListenForChannelUpdates() {
	go func() {
		cmd := m.GetListenCmd()
		for cmd != nil {
			msg := cmd()
			cmd, _ = msg.Apply(m)
		}
	}()
}

// An IO Operation, that will wait forever on msgChannel
func (m *Model) GetListenCmd() Cmd {
	return func() UpdateMsg {
		select {
		case <-m.bus.done:
			return stopListeningMsg{}
		case msg := <-m.msgChan:
			return msg
		case <-m.bus.wake:
		}
		m.bus.mu.Lock()
		delay := time.Until(m.bus.lastFlush.Add(progressInterval))
		m.bus.mu.Unlock()
		if delay > 0 {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-m.bus.done:
				return stopListeningMsg{}
			case msg := <-m.msgChan:
				m.wakeProgress()
				return msg
			case <-timer.C:
			}
		}
		m.bus.mu.Lock()
		updates := make([]Process, 0, len(m.bus.pending))
		for id, p := range m.bus.pending {
			updates = append(updates, p)
			delete(m.bus.pending, id)
		}
		m.bus.lastFlush = time.Now()
		m.bus.mu.Unlock()
		return progressBatchMsg{processes: updates}
	}
}

// Might add options to drain the channel in case msg is high priority.
func (m *Model) trySendMsgToChannel(msg UpdateMsg) error {
	select {
	case <-m.bus.done:
		return context.Canceled
	case m.msgChan <- msg:
		return nil
	default:
		// Process queue full with messages. Cannot add new process
		return &ProcessChannelFullError{}
	}
}

// Block till message is sent
func (m *Model) sendMsgToChannelBlocking(msg UpdateMsg) {
	select {
	case <-m.bus.done:
	case m.msgChan <- msg:
	}
}

func (m *Model) sendMsgToChannel(msg UpdateMsg, blocking bool) error {
	if blocking {
		select {
		case <-m.bus.done:
			return context.Canceled
		case m.msgChan <- msg:
			return nil
		}
	}
	return m.trySendMsgToChannel(msg)
}

func (m *Model) SendAddProcessMsg(
	currentFile string, operation OperationType, total int, blockingSend bool,
) (Process, error) {
	id := m.newUUIDForProcess()
	p := NewProcess(id, currentFile, operation, total)
	ctx, cancel := context.WithCancel(m.bus.lifetime)
	if operation == OpCreate {
		cancel()
		ctx, cancel = context.Background(), nil
	}
	m.bus.mu.Lock()
	m.bus.controls[p.ID] = taskControl{ctx: ctx, cancel: cancel, owned: true}
	m.bus.mu.Unlock()
	msg := newProcessMsg{
		NewProcess: p,
		BaseMsg:    BaseMsg{reqID: m.newReqCnt()},
	}
	err := m.sendMsgToChannel(msg, blockingSend)
	if err != nil {
		m.finishControl(p.ID)
		// Return zero-value process to indicate failure
		return Process{}, err
	}
	return p, nil
}

func (m *Model) SendUpdateProcessMsg(p Process, blockingSend bool) error {
	msg := updateProcessMsg{NewProcess: p, BaseMsg: BaseMsg{reqID: m.newReqCnt()}}
	return m.sendMsgToChannel(msg, blockingSend)
}

// Non Blocking and can fail
func (m *Model) TrySendingUpdateProcessMsg(p Process) {
	m.bus.mu.Lock()
	select {
	case <-m.bus.done:
		m.bus.mu.Unlock()
		return
	default:
	}
	m.bus.pending[p.ID] = p
	m.bus.mu.Unlock()
	m.wakeProgress()
}

func (m *Model) wakeProgress() {
	select {
	case m.bus.wake <- struct{}{}:
	default:
	}
}

type progressBatchMsg struct {
	BaseMsg

	processes []Process
}

func (msg progressBatchMsg) Apply(m *Model) (Cmd, error) {
	for _, p := range msg.processes {
		_ = m.UpdateExistingProcess(p)
	}
	return m.GetListenCmd(), nil
}

func (m *Model) SendStopListeningMsgBlocking() {
	m.sendMsgToChannelBlocking(stopListeningMsg{BaseMsg: BaseMsg{reqID: m.newReqCnt()}})
}
