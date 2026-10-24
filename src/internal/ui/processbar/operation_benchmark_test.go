package processbar

import (
	"strconv"
	"testing"
	"time"
)

// Identical probes also run against the untouched v0.1.0 worktree.
func BenchmarkOperationProgressBurst(b *testing.B) {
	m := New()
	p := NewProcess("bench", "file", OpCopy, b.N+1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		p.Done = i
		m.TrySendingUpdateProcessMsg(p)
	}
}

func BenchmarkOperationOrdering(b *testing.B) {
	m := New()
	for i := range 40 {
		p := NewProcess(strconv.Itoa(i), "file", OpCopy, 1)
		p.State = Successful
		p.DoneTime = time.Unix(int64(i), 0)
		m.AddOrUpdateProcess(p)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = m.getSortedProcesses()
	}
}
