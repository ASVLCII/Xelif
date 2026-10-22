package remote

import (
	"context"
	"os/exec"
	"sync"
)

type commandResult struct {
	stdout         []byte
	stderr         []byte
	err            error
	outputExceeded bool
}

type commandRunner interface {
	Run(ctx context.Context, binary string, args []string, stdoutLimit, stderrLimit int64) commandResult
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, binary string, args []string, stdoutLimit, stderrLimit int64) commandResult {
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stdout := &limitedBuffer{max: stdoutLimit, onLimit: cancel}
	stderr := &limitedBuffer{max: stderrLimit, onLimit: cancel}
	//nolint:gosec // G702: Client supplies only LookPath("rclone") output and direct argv; no shell is invoked.
	cmd := exec.CommandContext(runCtx, binary, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return commandResult{
		stdout:         stdout.Bytes(),
		stderr:         stderr.Bytes(),
		err:            err,
		outputExceeded: stdout.Exceeded() || stderr.Exceeded(),
	}
}

type limitedBuffer struct {
	mu       sync.Mutex
	data     []byte
	max      int64
	exceeded bool
	onLimit  func()
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.max - int64(len(b.data))
	if remaining > 0 {
		keep := int64(len(data))
		if keep > remaining {
			keep = remaining
		}
		b.data = append(b.data, data[:keep]...)
	}
	if int64(len(data)) > remaining {
		first := !b.exceeded
		b.exceeded = true
		if first && b.onLimit != nil {
			b.onLimit()
		}
	}
	return len(data), nil
}

func (b *limitedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.data...)
}

func (b *limitedBuffer) Exceeded() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.exceeded
}
