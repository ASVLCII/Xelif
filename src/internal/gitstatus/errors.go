package gitstatus

import (
	"fmt"
	"strings"
)

// ErrorKind permits callers to handle expected failures without matching
// command output.
type ErrorKind string

const (
	InvalidPath    ErrorKind = "invalid_path"
	NotRepository  ErrorKind = "not_repository"
	GitUnavailable ErrorKind = "git_unavailable"
	TimedOut       ErrorKind = "timed_out"
	OutputLimit    ErrorKind = "output_limit"
	InvalidOutput  ErrorKind = "invalid_output"
	CommandFailed  ErrorKind = "command_failed"
)

// Error is an actionable failure from a Git operation. Stderr is bounded and
// retained for diagnostics; Err supports errors.Is and errors.As.
type Error struct {
	Op     string
	Path   string
	Kind   ErrorKind
	Err    error
	Stderr string
}

func (e *Error) Error() string {
	message := fmt.Sprintf("gitstatus: %s failed for %q (%s)", e.Op, e.Path, e.Kind)
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	stderr := strings.TrimSpace(e.Stderr)
	stderrAlreadyIncluded := e.Err != nil && strings.Contains(e.Err.Error(), stderr)
	if stderr != "" && !stderrAlreadyIncluded {
		message += ": " + stderr
	}
	return message
}

func (e *Error) Unwrap() error { return e.Err }

func newError(op, path string, kind ErrorKind, err error, stderr string) *Error {
	return &Error{Op: op, Path: path, Kind: kind, Err: err, Stderr: stderr}
}
