package globalsearch

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

const commandWaitDelay = 100 * time.Millisecond

var errEverythingOutputLimit = errors.New("everything CLI output exceeded the safety limit")

// The CLI is optional, but its output must be bounded like native IPC replies.
func runEverythingCommand(ctx context.Context, executable string, args ...string) ([]byte, error) {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	output := searchCommandOutput{limit: ipcMaxReplyBytes, onLimit: cancel}
	cmd := exec.CommandContext(runCtx, executable, args...)
	cmd.Stdout = &output
	cmd.WaitDelay = commandWaitDelay
	err := cmd.Run()
	if output.exceeded {
		return nil, errEverythingOutputLimit
	}
	return output.data, err
}

const (
	indexedOutputLimit = 1024 * 1024
	indexedPathLimit   = 32 * 1024
	indexedStderrLimit = 4096
)

// NUL-delimited native output is stopped at a complete extra result, byte cap
// or oversized path. CommandContext kills obsolete requests immediately;
// WaitDelay also bounds waits for inherited pipes. No shell is involved.
func runIndexedCommand(ctx context.Context, executable string, args []string, limit int) indexedCommandResult {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	output := nulCommandOutput{limit: indexedOutputLimit, recordLimit: indexedPathLimit,
		resultLimit: boundedEverythingLimit(limit) + 1, onLimit: cancel}
	stderr := searchCommandOutput{limit: indexedStderrLimit}
	cmd := exec.CommandContext(runCtx, executable, args...)
	cmd.Stdout, cmd.Stderr = &output, &stderr
	cmd.WaitDelay = commandWaitDelay
	err := cmd.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	} else if output.truncated {
		// The child was killed by our own safety bound, not by a provider
		// failure. Preserve complete records and report a partial response.
		err = nil
	}
	return indexedCommandResult{Output: output.data, Stderr: string(stderr.data), Truncated: output.truncated, Err: err}
}

type nulCommandOutput struct {
	data        []byte
	limit       int
	recordLimit int
	resultLimit int
	recordBytes int
	results     int
	truncated   bool
	onLimit     func()
}

func (output *nulCommandOutput) stop() {
	if output.truncated {
		return
	}
	output.truncated = true
	// Drop an incomplete tail; it must not become a selectable fake path.
	if output.recordBytes > 0 {
		output.data = output.data[:len(output.data)-output.recordBytes]
		output.recordBytes = 0
	}
	if output.onLimit != nil {
		output.onLimit()
	}
}

func (output *nulCommandOutput) Write(data []byte) (int, error) {
	n := len(data)
	if output.truncated {
		return n, nil
	}
	for len(data) > 0 {
		end := bytes.IndexByte(data, 0)
		if end < 0 {
			end = len(data)
		} else {
			end++
		}
		piece := data[:end]
		if len(output.data)+len(piece) > output.limit || output.recordBytes+len(piece) > output.recordLimit {
			output.stop()
			return n, nil
		}
		output.data = append(output.data, piece...)
		output.recordBytes += len(piece)
		data = data[end:]
		if piece[len(piece)-1] == 0 {
			output.results++
			output.recordBytes = 0
			if output.results >= output.resultLimit {
				output.stop()
				return n, nil
			}
		}
	}
	return n, nil
}

// exec.Cmd has one stdout-copy goroutine and Wait joins it before data is read.
type searchCommandOutput struct {
	data     []byte
	limit    int
	exceeded bool
	onLimit  func()
}

func (output *searchCommandOutput) Write(data []byte) (int, error) {
	remaining := max(0, output.limit-len(output.data))
	output.data = append(output.data, data[:min(remaining, len(data))]...)
	if len(data) > remaining && !output.exceeded {
		output.exceeded = true
		if output.onLimit != nil {
			output.onLimit()
		}
	}
	return len(data), nil
}
