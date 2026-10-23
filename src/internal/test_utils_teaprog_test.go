package internal

import (
	"runtime"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

func TestTeaProgSendAcknowledgesUpdate(t *testing.T) {
	m := defaultTestModel(t.TempDir())
	p := NewTestTeaProgWithEventLoop(t, m)
	for width := 200; width < 220; width++ {
		p.Send(tea.WindowSizeMsg{Width: width, Height: 60})
		// Only these resize messages write fullWidth. Send must establish a
		// happens-before edge even without a second synchronized inspection.
		require.Equal(t, width, m.fullWidth)
	}
}

func TestTeaProgWithModelReleasesAfterGoexit(t *testing.T) {
	p := NewTestTeaProgWithEventLoop(t, defaultTestModel(t.TempDir()))
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		p.WithModel(func(m *model) {
			m.toggleFooter = true
			// testing.Fatal exits its caller through the same Goexit mechanism.
			runtime.Goexit()
		})
	}()
	<-exited
	p.Send(tea.WindowSizeMsg{Width: 240, Height: 60})
	require.True(t, p.Check(func(m *model) bool { return m.toggleFooter && m.fullWidth == 240 }))
	p.Close()
	select {
	case <-p.done:
	default:
		t.Fatal("Close returned before the event loop exited")
	}
}

func TestTeaProgCloseDrainsCommands(t *testing.T) {
	p := NewTestTeaProgWithEventLoop(t, defaultTestModel(t.TempDir()))
	started, finish := make(chan struct{}), make(chan struct{})
	var command tea.Cmd
	p.WithModel(func(*model) {
		command = p.owner.track(func() tea.Msg {
			close(started)
			<-finish
			return nil
		})
	})
	go func() { p.prog.Send(command()) }()
	<-started
	closed := make(chan struct{})
	go func() { defer close(closed); p.Close() }()
	require.Eventually(t, func() bool {
		return p.Check(func(*model) bool { return p.owner.closing })
	}, DefaultTestTimeout, DefaultTestTick)
	select {
	case <-closed:
		t.Fatal("Close returned while a command was still running")
	default:
	}
	close(finish)
	<-closed
	require.Zero(t, p.owner.pending)
}
