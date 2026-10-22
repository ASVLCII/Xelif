package remote

import (
	"fmt"
	"strings"
)

// ErrorKind identifies expected remote failures without requiring callers to
// inspect rclone output.
type ErrorKind string

const (
	Unavailable    ErrorKind = "unavailable"
	NotConfigured  ErrorKind = "not_configured"
	Authentication ErrorKind = "authentication"
	TimedOut       ErrorKind = "timed_out"
	Cancelled      ErrorKind = "cancelled"
	OutputLimit    ErrorKind = "output_limit"
	InvalidOutput  ErrorKind = "invalid_output"
	InvalidPath    ErrorKind = "invalid_path"
	AlreadyExists  ErrorKind = "already_exists"
	CommandFailed  ErrorKind = "command_failed"
)

// Error is a typed failure from a remote operation. Stderr is always bounded.
// Err is available through errors.Is and errors.As.
type Error struct {
	Op         string
	RemotePath string
	Kind       ErrorKind
	Err        error
	Stderr     string
}

func (e *Error) Error() string {
	message := fmt.Sprintf("remote: %s failed", e.Op)
	if e.RemotePath != "" {
		message += fmt.Sprintf(" for %q", e.RemotePath)
	}
	message += fmt.Sprintf(" (%s)", e.Kind)
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	stderr := strings.TrimSpace(e.Stderr)
	if stderr != "" && (e.Err == nil || !strings.Contains(e.Err.Error(), stderr)) {
		message += ": " + stderr
	}
	return message
}

func (e *Error) Unwrap() error { return e.Err }

func newError(op, remotePath string, kind ErrorKind, err error, stderr string) *Error {
	return &Error{Op: op, RemotePath: remotePath, Kind: kind, Err: err, Stderr: stderr}
}
