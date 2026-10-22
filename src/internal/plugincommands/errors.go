package plugincommands

import (
	"context"
	"fmt"
	"time"
)

// DiscoveryError reports an unsafe or unreadable manifest directory.
type DiscoveryError struct {
	Dir string
	Err error
}

func (e *DiscoveryError) Error() string {
	return fmt.Sprintf("discover command plugins in %q: %v", e.Dir, e.Err)
}
func (e *DiscoveryError) Unwrap() error { return e.Err }

// ManifestError reports a malformed or unsafe manifest.
type ManifestError struct {
	File  string
	Field string
	Err   error
}

func (e *ManifestError) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("invalid command-plugin manifest %q: %v", e.File, e.Err)
	}
	return fmt.Sprintf("invalid command-plugin manifest %q field %q: %v", e.File, e.Field, e.Err)
}
func (e *ManifestError) Unwrap() error { return e.Err }

// DuplicateIDError reports two manifests declaring the same stable ID.
type DuplicateIDError struct {
	ID         string
	FirstFile  string
	SecondFile string
}

func (e *DuplicateIDError) Error() string {
	return fmt.Sprintf("duplicate command-plugin ID %q in %q and %q", e.ID, e.FirstFile, e.SecondFile)
}

// NotFoundError reports an execution request for an unknown plugin.
type NotFoundError struct{ PluginID string }

func (e *NotFoundError) Error() string { return fmt.Sprintf("command plugin %q not found", e.PluginID) }

// SelectionError reports an empty, relative, or unclean selection/cwd.
type SelectionError struct{ Reason string }

func (e *SelectionError) Error() string { return "invalid command-plugin selection: " + e.Reason }

// ExtensionError reports a selected path rejected by a plugin's extension list.
type ExtensionError struct {
	PluginID string
	Path     string
}

func (e *ExtensionError) Error() string {
	return fmt.Sprintf("command plugin %q does not accept selected path %q", e.PluginID, e.Path)
}

// StartError reports failure to start the configured executable.
type StartError struct {
	PluginID string
	Err      error
}

func (e *StartError) Error() string {
	return fmt.Sprintf("start command plugin %q: %v", e.PluginID, e.Err)
}
func (e *StartError) Unwrap() error { return e.Err }

// TimeoutError reports enforcement of a manifest's execution timeout.
type TimeoutError struct {
	PluginID string
	Timeout  time.Duration
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("command plugin %q exceeded timeout %s", e.PluginID, e.Timeout)
}
func (e *TimeoutError) Unwrap() error { return context.DeadlineExceeded }

// CanceledError reports cancellation by the caller.
type CanceledError struct {
	PluginID string
	Err      error
}

func (e *CanceledError) Error() string {
	return fmt.Sprintf("command plugin %q canceled: %v", e.PluginID, e.Err)
}
func (e *CanceledError) Unwrap() error { return e.Err }

// OutputLimitError reports enforcement of a manifest's combined output bound.
type OutputLimitError struct {
	PluginID string
	Limit    int
}

func (e *OutputLimitError) Error() string {
	return fmt.Sprintf("command plugin %q exceeded combined output limit %d", e.PluginID, e.Limit)
}

// ExitError reports a process that started successfully but exited non-zero.
type ExitError struct {
	PluginID string
	ExitCode int
	Err      error
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("command plugin %q exited with code %d", e.PluginID, e.ExitCode)
}
func (e *ExitError) Unwrap() error { return e.Err }
