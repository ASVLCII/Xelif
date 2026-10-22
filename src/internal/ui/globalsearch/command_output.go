package globalsearch

import (
	"context"
	"errors"
	"os/exec"
)

var errEverythingOutputLimit = errors.New("everything CLI output exceeded the safety limit")

// The CLI is optional, but its output must be bounded like native IPC replies.
func runEverythingCommand(ctx context.Context, executable string, args ...string) ([]byte, error) {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	output := searchCommandOutput{limit: ipcMaxReplyBytes, onLimit: cancel}
	cmd := exec.CommandContext(runCtx, executable, args...)
	cmd.Stdout = &output
	err := cmd.Run()
	if output.exceeded {
		return nil, errEverythingOutputLimit
	}
	return output.data, err
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
