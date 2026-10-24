package processbar

import (
	"sort"

	"github.com/lithammer/shortuuid"
)

const maxCompletedProcesses = 100

func (m *Model) cntProcesses() int {
	return len(m.getFooterProcesses())
}

func (m *Model) getFooterProcesses() []Process {
	processes := m.getSortedProcesses()
	if m.bus == nil {
		return processes
	}
	m.bus.mu.Lock()
	defer m.bus.mu.Unlock()
	return m.bus.footer
}

func (m *Model) isValid() bool {
	return m.renderIndex <= m.cursor &&
		m.cursor <= m.renderIndex+cntRenderableProcess(m.height-borderSize)-1
}

func (m *Model) viewHeight() int {
	return m.height - borderSize
}

func (m *Model) viewWidth() int {
	return m.width - borderSize
}

func (m *Model) GetHeight() int {
	return m.height
}

func (m *Model) GetWidth() int {
	return m.width
}

func (m *Model) getSortedProcesses() []Process {
	m.processesMu.RLock()
	defer m.processesMu.RUnlock()
	if m.bus == nil {
		return m.sortedSnapshot()
	}
	m.bus.mu.Lock()
	defer m.bus.mu.Unlock()
	if m.bus.dirty {
		m.bus.ordered = m.sortedSnapshot()
		m.bus.footer = make([]Process, 0, len(m.bus.ordered))
		for _, p := range m.bus.ordered {
			if !p.Background {
				m.bus.footer = append(m.bus.footer, p)
			}
		}
		m.bus.dirty = false
	}
	return m.bus.ordered
}

// Caller owns processesMu; returned snapshots are immutable after publication.
func (m *Model) sortedSnapshot() []Process {
	processes := make([]Process, 0, len(m.processes))
	for _, p := range m.processes {
		processes = append(processes, p)
	}
	sort.Slice(processes, func(i, j int) bool { return processBefore(&processes[i], &processes[j]) })
	return processes
}

func processBefore(a, b *Process) bool {
	activeA, activeB := a.DoneTime.IsZero(), b.DoneTime.IsZero()
	if activeA != activeB {
		return activeA
	}
	if !a.DoneTime.Equal(b.DoneTime) {
		return a.DoneTime.After(b.DoneTime)
	}
	if !a.Started.Equal(b.Started) {
		return a.Started.Before(b.Started)
	}
	return a.ID < b.ID
}

func (m *Model) newReqCnt() int {
	return int(m.reqCnt.Add(1))
}

// TODO: Maybe make sure that there isn't any existing process with this UUID
func (m *Model) newUUIDForProcess() string {
	return shortuuid.New()
}

// Copy of the current processes for read only purpose
func (m *Model) GetProcessesSlice() []Process {
	m.processesMu.RLock()
	defer m.processesMu.RUnlock()

	var processes []Process
	for _, p := range m.processes {
		processes = append(processes, p)
	}
	return processes
}

func (m *Model) pruneCompletedProcesses() {
	// The caller holds processesMu for writing so pruning is atomic with the
	// update that triggered it.
	completed := make([]Process, 0, len(m.processes))
	for _, process := range m.processes {
		if process.State != InOperation && !process.DoneTime.IsZero() {
			completed = append(completed, process)
		}
	}
	if len(completed) <= maxCompletedProcesses {
		return
	}

	sort.Slice(completed, func(i, j int) bool {
		return completed[i].DoneTime.Before(completed[j].DoneTime)
	})
	for _, process := range completed[:len(completed)-maxCompletedProcesses] {
		delete(m.processes, process.ID)
	}
	if len(m.processes) == 0 {
		m.cursor = 0
		m.renderIndex = 0
		return
	}
	m.cursor = min(m.cursor, len(m.processes)-1)
	m.renderIndex = min(m.renderIndex, m.cursor)
}
