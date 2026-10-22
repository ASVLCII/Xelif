// Package duplicates finds files with identical content without modifying the
// file system.
package duplicates

import (
	"errors"
	"fmt"
)

// ErrorCode classifies fatal errors caused by invalid scan input.
type ErrorCode string

const (
	ErrorInvalidRoots   ErrorCode = "invalid_roots"
	ErrorInvalidRoot    ErrorCode = "invalid_root"
	ErrorInvalidOptions ErrorCode = "invalid_options"
)

// ScanError is returned for invalid input that prevents a scan from starting.
// Runtime file-system failures below a valid root are reported as Warnings.
type ScanError struct {
	Code ErrorCode
	Path string
	Err  error
}

func (e *ScanError) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("duplicate scan %s: %v", e.Code, e.Err)
	}
	return fmt.Sprintf("duplicate scan %s for %q: %v", e.Code, e.Path, e.Err)
}

// Unwrap exposes the underlying validation or file-system error.
func (e *ScanError) Unwrap() error { return e.Err }

// Operation identifies the scan step that produced a non-fatal warning.
type Operation string

const (
	OperationStat    Operation = "stat"
	OperationReadDir Operation = "read_dir"
	OperationOpen    Operation = "open"
	OperationRead    Operation = "read"
	OperationLimit   Operation = "limit"
)

var (
	// ErrMaxFilesReached indicates that MaxFiles stopped discovery.
	ErrMaxFilesReached = errors.New("maximum file count reached")
	// ErrHashBudgetReached indicates that MaxBytesHashed prevented a candidate
	// from being hashed.
	ErrHashBudgetReached = errors.New("maximum hash byte budget reached")
)

// WarningError describes a recoverable problem. Scan skips the affected path
// and continues whenever possible.
type WarningError struct {
	Path      string
	Operation Operation
	Err       error
}

func (w WarningError) Error() string {
	if w.Path == "" {
		return fmt.Sprintf("%s: %v", w.Operation, w.Err)
	}
	return fmt.Sprintf("%s %q: %v", w.Operation, w.Path, w.Err)
}

// Unwrap exposes the underlying file-system or limit error.
func (w WarningError) Unwrap() error { return w.Err }

// Warning is retained as an alias for compatibility.
type Warning = WarningError //nolint:errname // Preserve the existing public alias; new code uses WarningError.

// Phase is the kind of work described by a Progress event.
type Phase string

const (
	PhaseDiscovered Phase = "discovered"
	PhaseHashed     Phase = "hashed"
)

// Progress is a best-effort scan update. Events may be dropped so a slow or
// absent receiver can never stall discovery or hashing workers.
type Progress struct {
	Phase       Phase
	Path        string
	Files       int
	BytesHashed int64
}

// Options controls resource use and traversal behavior. Zero limits are
// unlimited. A zero Workers value selects a runtime-derived positive default.
// Symlinks and hidden entries are skipped unless explicitly enabled.
type Options struct {
	MaxFiles       int
	MaxBytesHashed int64
	Workers        int
	FollowSymlinks bool
	IncludeHidden  bool
	// IncludeHardLinks treats every hard-link path as a separate file. The
	// default counts one path per underlying file identity.
	IncludeHardLinks bool
	// Progress receives best-effort non-blocking updates. Scan never closes a
	// caller-owned channel.
	Progress chan<- Progress
}

// File is one member of a duplicate group.
type File struct {
	Path   string
	Size   int64
	Digest string
}

// Group contains two or more files with the same size and SHA-256 digest.
type Group struct {
	Size   int64
	Digest string
	Files  []File
}

// Result is a deterministic snapshot of a scan. Truncated is set when a
// resource bound prevented a complete result.
type Result struct {
	Groups       []Group
	Warnings     []Warning
	FilesScanned int
	FilesHashed  int
	BytesHashed  int64
	Truncated    bool
}
