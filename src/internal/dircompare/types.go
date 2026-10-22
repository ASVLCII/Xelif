// Package dircompare compares directory trees and safely copies entries that
// are missing from another tree. It never mirrors or deletes entries.
package dircompare

import (
	"fmt"
	"io/fs"
	"time"
)

const (
	defaultMaxEntries = 100_000
	maximumMaxEntries = 1_000_000
	defaultMaxDepth   = 128
	maximumMaxDepth   = 1_024
	tempPrefix        = ".dircompare-"
	fileBufferSize    = 64 * 1024
	optionMaxEntries  = "MaxEntries"
	optionMaxDepth    = "MaxDepth"
)

// Classification describes the relationship between entries at one path.
type Classification string

const (
	OnlyLeft     Classification = "only-left"
	OnlyRight    Classification = "only-right"
	Same         Classification = "same"
	Different    Classification = "different"
	TypeMismatch Classification = "type-mismatch"
)

// EntryType is the filesystem type of an entry.
type EntryType string

const (
	TypeRegular   EntryType = "regular-file"
	TypeDirectory EntryType = "directory"
	TypeSymlink   EntryType = "symlink"
	TypeOther     EntryType = "other"
)

// Side identifies one of the compared roots.
type Side string

const (
	LeftSide  Side = "left"
	RightSide Side = "right"
)

// Metadata is read-only information captured for an entry.
type Metadata struct {
	Exists     bool
	Type       EntryType
	Size       int64
	Mode       fs.FileMode
	ModTime    time.Time
	LinkTarget string
}

// Entry is one deterministic, slash-separated relative-path result.
type Entry struct {
	Path           string
	Classification Classification
	Left           Metadata
	Right          Metadata
}

// WarningError reports a non-fatal problem encountered while comparing a tree.
type WarningError struct {
	Side Side
	Path string
	Err  error
}

func (w WarningError) Error() string {
	if w.Path == "" {
		return fmt.Sprintf("%s tree: %v", w.Side, w.Err)
	}
	return fmt.Sprintf("%s tree %q: %v", w.Side, w.Path, w.Err)
}

func (w WarningError) Unwrap() error { return w.Err }

// Warning is retained as an alias for compatibility.
type Warning = WarningError //nolint:errname // Preserve the existing public alias; new code uses WarningError.

// Result contains sorted comparison entries and warnings.
type Result struct {
	Entries  []Entry
	Warnings []Warning
}

// Options controls comparison. Zero values select safe bounded defaults.
type Options struct {
	// VerifyContent compares equal-sized regular files using SHA-256 instead
	// of trusting modification times.
	VerifyContent bool
	// MaxEntries bounds the total number of entries visited across both trees.
	MaxEntries int
	// MaxDepth bounds traversal below each root. Direct children have depth 1.
	MaxDepth int
}

// ActionKind describes a copy plan action.
type ActionKind string

const (
	CreateDirectory ActionKind = "create-directory"
	CopyFile        ActionKind = "copy-file"
)

// CopyAction is an entry that CopyMissing plans to create.
type CopyAction struct {
	Path string
	Kind ActionKind
	Mode fs.FileMode
}

// Progress is reported synchronously after an action is completed.
type Progress struct {
	Completed int
	Total     int
	Action    CopyAction
}

// CopyOptions controls CopyMissing. Zero values select safe bounded defaults.
type CopyOptions struct {
	DryRun     bool
	MaxEntries int
	MaxDepth   int
	Progress   func(Progress)
	// ApprovedPlan binds this invocation to an unchanged DryRun result. A stale
	// or caller-modified preview fails before writes; new actions are never added.
	ApprovedPlan *CopyResult
}

// CopyResult contains the deterministic plan and an audit of paths created.
// Created records successful creation events as slash-separated relative paths,
// even on failure. It is not an inventory of their current contents or identity:
// another actor may edit, replace, or remove them after publication.
type CopyResult struct {
	Actions []CopyAction
	Created []string
	// Deprecated: completed destination entries are retained on failure.
	// RolledBack is always false.
	RolledBack bool

	// Only a successful dry run carries an immutable, content-verified approval.
	approval *copyApproval
}

// StalePlanError requires a fresh preview before a copy can be approved again.
// No destination creations occur when freshness validation fails before apply;
// a later source change can instead be wrapped by PartialCopyError.
type StalePlanError struct {
	Path string
	Err  error
}

func (e *StalePlanError) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("dircompare: copy-missing preview is stale; preview again before applying: %v", e.Err)
	}
	return fmt.Sprintf(
		"dircompare: copy-missing preview is stale at %q; preview again before applying: %v",
		e.Path,
		e.Err,
	)
}

func (e *StalePlanError) Unwrap() error { return e.Err }

// PartialCopyError reports a failure after at least one destination creation.
// Created is an independent copy of the creation audit in CopyResult, containing
// slash-separated relative paths. CopyMissing does not remove published entries
// on failure; their subsequent contents and identity are not guaranteed.
type PartialCopyError struct {
	Created []string
	Err     error
}

func (e *PartialCopyError) Error() string {
	return fmt.Sprintf("dircompare: completed destination entries retained (%d): %v", len(e.Created), e.Err)
}

func (e *PartialCopyError) Unwrap() error { return e.Err }

// OptionError reports an invalid or unbounded caller option.
type OptionError struct {
	Option string
	Value  int
	Reason string
}

func (e *OptionError) Error() string {
	return fmt.Sprintf("dircompare: invalid %s value %d: %s", e.Option, e.Value, e.Reason)
}

// LimitExceededError reports that bounded traversal was stopped.
type LimitExceededError struct {
	Option string
	Limit  int
	Path   string
}

func (e *LimitExceededError) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("dircompare: %s limit %d exceeded", e.Option, e.Limit)
	}
	return fmt.Sprintf("dircompare: %s limit %d exceeded at %q", e.Option, e.Limit, e.Path)
}

// RootError reports an invalid directory root.
type RootError struct {
	Root string
	Err  error
}

func (e *RootError) Error() string {
	return fmt.Sprintf("dircompare: invalid root %q: %v", e.Root, e.Err)
}
func (e *RootError) Unwrap() error { return e.Err }

// PathError reports an operation failure for a relative path.
type PathError struct {
	Op   string
	Path string
	Err  error
}

func (e *PathError) Error() string {
	return fmt.Sprintf("dircompare: %s %q: %v", e.Op, e.Path, e.Err)
}
func (e *PathError) Unwrap() error { return e.Err }

// CollisionError reports incompatible entries at the same destination path.
type CollisionError struct {
	Path            string
	SourceType      EntryType
	DestinationType EntryType
}

func (e *CollisionError) Error() string {
	return fmt.Sprintf(
		"dircompare: destination collision at %q: source is %s, destination is %s",
		e.Path,
		e.SourceType,
		e.DestinationType,
	)
}

// UnsupportedTypeError reports an entry CopyMissing will not copy.
type UnsupportedTypeError struct {
	Path string
	Type EntryType
}

func (e *UnsupportedTypeError) Error() string {
	return fmt.Sprintf("dircompare: refusing to copy unsupported %s at %q", e.Type, e.Path)
}

// RootOverlapError reports roots that could recursively copy into themselves.
type RootOverlapError struct {
	Source      string
	Destination string
}

func (e *RootOverlapError) Error() string {
	return fmt.Sprintf("dircompare: source %q and destination %q must not overlap", e.Source, e.Destination)
}

// RollbackError reports failures while removing items created by an invocation.
//
// Deprecated: CopyMissing retains published entries and no longer rolls back.
type RollbackError struct {
	Errors []error
}

func (e *RollbackError) Error() string {
	return fmt.Sprintf("dircompare: rollback failed for %d item(s)", len(e.Errors))
}

func (e *RollbackError) Unwrap() []error { return e.Errors }

type boundedOptions struct {
	maxEntries int
	maxDepth   int
}

func normalizeBounds(maxEntries, maxDepth int) (boundedOptions, error) {
	if maxEntries < 0 || maxEntries > maximumMaxEntries {
		return boundedOptions{}, &OptionError{
			Option: optionMaxEntries,
			Value:  maxEntries,
			Reason: fmt.Sprintf("must be between 0 and %d", maximumMaxEntries),
		}
	}
	if maxDepth < 0 || maxDepth > maximumMaxDepth {
		return boundedOptions{}, &OptionError{
			Option: optionMaxDepth,
			Value:  maxDepth,
			Reason: fmt.Sprintf("must be between 0 and %d", maximumMaxDepth),
		}
	}
	if maxEntries == 0 {
		maxEntries = defaultMaxEntries
	}
	if maxDepth == 0 {
		maxDepth = defaultMaxDepth
	}
	return boundedOptions{maxEntries: maxEntries, maxDepth: maxDepth}, nil
}
