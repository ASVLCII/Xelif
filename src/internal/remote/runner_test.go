package remote

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRealRunnerEnforcesOutputLimit(t *testing.T) {
	result := (execRunner{}).Run(context.Background(), os.Args[0], []string{
		"-test.run=TestRemoteHelperProcess", "--", "spam",
	}, 128, 128)
	if !result.outputExceeded {
		t.Fatalf("result = %#v", result)
	}
	if len(result.stdout) > 128 || len(result.stderr) > 128 {
		t.Fatalf("unbounded output: stdout=%d stderr=%d", len(result.stdout), len(result.stderr))
	}
}

func TestRealRunnerHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := (execRunner{}).Run(ctx, os.Args[0], []string{
		"-test.run=TestRemoteHelperProcess", "--", "wait",
	}, 128, 128)
	if !errors.Is(result.err, context.Canceled) && !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("error = %v", result.err)
	}
}

func TestClientClassifiesTimeoutAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		kind ErrorKind
	}{
		{name: "timeout", err: context.DeadlineExceeded, kind: TimedOut},
		{name: "cancel", err: context.Canceled, kind: Cancelled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{responses: []fakeResponse{{err: tc.err}}}
			_, err := testClient(runner).Remotes(context.Background())
			assertErrorKind(t, err, tc.kind)
			if !errors.Is(err, tc.err) {
				t.Fatalf("errors.Is(%v) = false", tc.err)
			}
		})
	}
}

func TestClientClassifiesBoundedOutput(t *testing.T) {
	runner := &fakeRunner{responses: []fakeResponse{{}}}
	runner.responses[0].run = func([]string) error { return nil }
	client := testClient(outputLimitRunner{runner})
	_, err := client.Remotes(context.Background())
	assertErrorKind(t, err, OutputLimit)
}

func TestClientClassifiesExecutionFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result fakeResponse
		kind   ErrorKind
	}{
		{
			name: "missing executable",
			result: fakeResponse{
				err: &exec.Error{Name: "rclone", Err: exec.ErrNotFound},
			},
			kind: Unavailable,
		},
		{
			name:   "ordinary failure",
			result: fakeResponse{stderr: "backend unavailable", err: errors.New("exit status 1")},
			kind:   CommandFailed,
		},
		{
			name:   "missing config",
			result: fakeResponse{stderr: "didn't find section in config file", err: errors.New("exit status 1")},
			kind:   NotConfigured,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{responses: []fakeResponse{tc.result}}
			_, err := testClient(runner).Remotes(context.Background())
			assertErrorKind(t, err, tc.kind)
		})
	}
}

type outputLimitRunner struct{ commandRunner }

func (r outputLimitRunner) Run(
	ctx context.Context,
	binary string,
	args []string,
	stdoutLimit int64,
	stderrLimit int64,
) commandResult {
	result := r.commandRunner.Run(ctx, binary, args, stdoutLimit, stderrLimit)
	result.outputExceeded = true
	return result
}

func TestRemoteHelperProcess(t *testing.T) {
	separator := -1
	for i, arg := range os.Args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) {
		return
	}
	switch os.Args[separator+1] {
	case "spam":
		_, _ = fmt.Fprint(os.Stdout, strings.Repeat("x", 4096))
	case "wait":
		time.Sleep(10 * time.Second)
	}
}
