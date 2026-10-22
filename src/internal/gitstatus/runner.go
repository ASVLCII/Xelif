package gitstatus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

var errOutputLimit = errors.New("git output exceeded the configured limit")

const maxStderrOutput = int64(64 << 10)

type limitedBuffer struct {
	data     []byte
	max      int64
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.max - int64(len(b.data))
	if remaining <= 0 {
		b.exceeded = true
		return len(p), nil
	}
	if int64(len(p)) > remaining {
		b.data = append(b.data, p[:remaining]...)
		b.exceeded = true
		return len(p), nil
	}
	b.data = append(b.data, p...)
	return len(p), nil
}

func (c *Client) run(ctx context.Context, operation, path, dir string, readOnly bool, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	bounded, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// c.gitBinary is initialized internally to "git" and args are passed
	// directly to the process without a shell. User paths are repository-relative
	// literal pathspecs placed after "--" by callers.
	//nolint:gosec // G204: binary and argv are constrained as documented above.
	cmd := exec.CommandContext(bounded, c.gitBinary, args...)
	cmd.Dir = dir
	if readOnly {
		cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	}
	stdout := &limitedBuffer{max: c.maxOutput}
	stderr := &limitedBuffer{max: maxStderrOutput}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if bounded.Err() != nil {
		return nil, newError(operation, path, TimedOut, bounded.Err(), string(stderr.data))
	}
	if stdout.exceeded || stderr.exceeded {
		return nil, newError(operation, path, OutputLimit, errOutputLimit, string(stderr.data))
	}
	if err == nil {
		return stdout.data, nil
	}
	var execErr *exec.Error
	if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
		return nil, newError(operation, path, GitUnavailable, err, string(stderr.data))
	}
	return nil, newError(operation, path, CommandFailed, fmt.Errorf("git command: %w", err), string(stderr.data))
}
