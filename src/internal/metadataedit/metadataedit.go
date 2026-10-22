// Package metadataedit safely previews, applies, and restores filesystem metadata.
// It never reads or writes file contents or embedded media metadata.
package metadataedit

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maximumPermissions = fs.FileMode(0o777)

// Metadata is the editable filesystem metadata for one local path.
type Metadata struct {
	Path                string
	ModifiedTime        time.Time
	AccessTime          time.Time
	Permissions         fs.FileMode
	PermissionsEditable bool
}

// Changes describes optional replacement values. Nil fields are preserved.
type Changes struct {
	ModifiedTime *time.Time
	AccessTime   *time.Time
	Permissions  *fs.FileMode
}

// Patch is a validated, immutable metadata change built by BuildPatch.
// Its fields are intentionally private so an invalid patch cannot be constructed.
type Patch struct {
	target  string
	changes patchChanges
}

type patchChanges struct {
	modifiedTime time.Time
	accessTime   time.Time
	permissions  fs.FileMode
	modified     bool
	accessed     bool
	permission   bool
}

// PreviewResult is the current and proposed metadata for a patch.
type PreviewResult struct {
	Before Metadata
	After  Metadata
}

// BuildPatch validates changes without touching the target.
func BuildPatch(path string, changes Changes) (Patch, error) {
	const op = "build patch"
	if path == "" || strings.IndexByte(path, 0) >= 0 {
		return Patch{}, newError(
			KindInvalidInput,
			op,
			path,
			errors.New("path must be non-empty and contain no NUL byte"),
		)
	}
	if changes.ModifiedTime == nil && changes.AccessTime == nil && changes.Permissions == nil {
		return Patch{}, newError(KindInvalidInput, op, path, errors.New("at least one change is required"))
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return Patch{}, newError(KindInvalidInput, op, path, err)
	}
	result := Patch{target: filepath.Clean(absPath)}
	if changes.ModifiedTime != nil {
		if err := validateTime(*changes.ModifiedTime); err != nil {
			return Patch{}, newError(KindInvalidInput, op, path, fmt.Errorf("modified time: %w", err))
		}
		result.changes.modifiedTime = changes.ModifiedTime.Round(0)
		result.changes.modified = true
	}
	if changes.AccessTime != nil {
		if err := validateTime(*changes.AccessTime); err != nil {
			return Patch{}, newError(KindInvalidInput, op, path, fmt.Errorf("access time: %w", err))
		}
		result.changes.accessTime = changes.AccessTime.Round(0)
		result.changes.accessed = true
	}
	if changes.Permissions != nil {
		if *changes.Permissions&^maximumPermissions != 0 {
			return Patch{}, newError(
				KindInvalidInput,
				op,
				path,
				errors.New("permissions must contain only Unix rwx bits (0000-0777)"),
			)
		}
		if !permissionsSupported {
			return Patch{}, newError(
				KindUnsupported,
				op,
				path,
				errors.New("permission changes are not supported on this platform"),
			)
		}
		result.changes.permissions = *changes.Permissions
		result.changes.permission = true
	}
	return result, nil
}

func validateTime(value time.Time) error {
	if value.IsZero() {
		return errors.New("zero time is not valid")
	}
	if _, err := value.MarshalText(); err != nil {
		return fmt.Errorf("time cannot be represented in the journal: %w", err)
	}
	return nil
}

// Inspect returns the editable filesystem metadata for path without following symlinks.
func Inspect(ctx context.Context, path string) (Metadata, error) {
	if err := checkContext(ctx, "inspect", path); err != nil {
		return Metadata{}, err
	}
	if path == "" || strings.IndexByte(path, 0) >= 0 {
		return Metadata{}, newError(
			KindInvalidInput,
			"inspect",
			path,
			errors.New("path must be non-empty and contain no NUL byte"),
		)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return Metadata{}, newError(KindInvalidInput, "inspect", path, err)
	}
	absPath = filepath.Clean(absPath)
	info, err := os.Lstat(absPath)
	if err != nil {
		return Metadata{}, newError(KindInspect, "inspect", absPath, err)
	}
	fileTypeErr := validateFileType(info.Mode())
	if fileTypeErr != nil {
		return Metadata{}, newError(KindUnsafeFile, "inspect", absPath, fileTypeErr)
	}
	accessed, err := fileAccessTime(info)
	if err != nil {
		return Metadata{}, newError(KindInspect, "inspect access time", absPath, err)
	}
	if err := checkContext(ctx, "inspect", absPath); err != nil {
		return Metadata{}, err
	}
	return Metadata{
		Path:                absPath,
		ModifiedTime:        info.ModTime().Round(0),
		AccessTime:          accessed.Round(0),
		Permissions:         info.Mode().Perm(),
		PermissionsEditable: permissionsSupported,
	}, nil
}

func validateFileType(mode fs.FileMode) error {
	if mode&fs.ModeSymlink != 0 {
		return errors.New("symbolic links are refused")
	}
	if !mode.IsRegular() && !mode.IsDir() {
		return fmt.Errorf("special file type %v is refused", mode.Type())
	}
	return nil
}

// Preview returns the current and proposed metadata without mutating the path.
func Preview(ctx context.Context, patch Patch) (PreviewResult, error) {
	if err := validatePatch(patch); err != nil {
		return PreviewResult{}, err
	}
	before, err := Inspect(ctx, patch.target)
	if err != nil {
		return PreviewResult{}, err
	}
	return PreviewResult{Before: before, After: proposedMetadata(before, patch.changes)}, nil
}

func validatePatch(patch Patch) error {
	if patch.target == "" || (!patch.changes.modified && !patch.changes.accessed && !patch.changes.permission) {
		return newError(
			KindInvalidInput,
			"validate patch",
			patch.target,
			errors.New("patch must be created by BuildPatch"),
		)
	}
	return nil
}

func proposedMetadata(before Metadata, changes patchChanges) Metadata {
	after := before
	if changes.modified {
		after.ModifiedTime = changes.modifiedTime
	}
	if changes.accessed {
		after.AccessTime = changes.accessTime
	}
	if changes.permission {
		after.Permissions = changes.permissions
	}
	return after
}

type changeSet struct {
	permissions bool
	times       bool
}

type fileOps struct {
	chmod   func(string, fs.FileMode) error
	chtimes func(string, time.Time, time.Time) error
}

func operatingSystemFileOps() fileOps {
	return fileOps{chmod: os.Chmod, chtimes: os.Chtimes}
}

// Apply atomically creates a private backup journal before mutating metadata.
func Apply(ctx context.Context, patch Patch, journalPath string) (PreviewResult, error) {
	return apply(ctx, patch, journalPath, operatingSystemFileOps())
}

func apply(ctx context.Context, patch Patch, journalPath string, ops fileOps) (PreviewResult, error) {
	if err := validatePatch(patch); err != nil {
		return PreviewResult{}, err
	}
	if err := checkContext(ctx, "apply", patch.target); err != nil {
		return PreviewResult{}, err
	}
	journalPath, err := validateJournalPath(journalPath, patch.target)
	if err != nil {
		return PreviewResult{}, err
	}
	preview, err := Preview(ctx, patch)
	if err != nil {
		return PreviewResult{}, err
	}
	j := journalFromPreview(preview)
	journalErr := createJournal(journalPath, j)
	if journalErr != nil {
		return PreviewResult{}, journalErr
	}
	contextErr := checkContext(ctx, "apply", patch.target)
	if contextErr != nil {
		return PreviewResult{}, contextErr
	}
	changes := changeSet{
		permissions: patch.changes.permission,
		times:       patch.changes.modified || patch.changes.accessed,
	}
	mutationErr := applyChanges(ctx, patch.target, preview.Before, preview.After, changes, ops)
	if mutationErr != nil {
		return PreviewResult{}, mutationErr
	}
	actual, err := Inspect(ctx, patch.target)
	if err != nil {
		rollbackErr := rollbackChanges(patch.target, preview.Before, changes, ops)
		if rollbackErr != nil {
			return PreviewResult{}, newError(
				KindRollback,
				"apply verification",
				patch.target,
				errors.Join(err, rollbackErr),
			)
		}
		return PreviewResult{}, err
	}
	return PreviewResult{Before: preview.Before, After: actual}, nil
}

func applyChanges(
	ctx context.Context,
	target string,
	before, after Metadata,
	changes changeSet,
	ops fileOps,
) error {
	if err := checkContext(ctx, "mutate", target); err != nil {
		return err
	}
	applied := changeSet{}
	if changes.permissions {
		if err := ops.chmod(target, after.Permissions); err != nil {
			return newError(KindMutation, "set permissions", target, err)
		}
		applied.permissions = true
		if err := checkContext(ctx, "mutate", target); err != nil {
			return rollbackApplied(target, before, applied, ops, err)
		}
	}
	if changes.times {
		if err := ops.chtimes(target, after.AccessTime, after.ModifiedTime); err != nil {
			mutationErr := newError(KindMutation, "set timestamps", target, err)
			return rollbackApplied(target, before, applied, ops, mutationErr)
		}
		applied.times = true
		if err := checkContext(ctx, "mutate", target); err != nil {
			return rollbackApplied(target, before, applied, ops, err)
		}
	}
	return nil
}

func rollbackApplied(target string, before Metadata, applied changeSet, ops fileOps, cause error) error {
	rollbackErr := rollbackChanges(target, before, applied, ops)
	if rollbackErr != nil {
		return newError(KindRollback, "rollback", target, errors.Join(cause, rollbackErr))
	}
	return cause
}

func rollbackChanges(target string, before Metadata, changes changeSet, ops fileOps) error {
	var failures []error
	if changes.times {
		if err := ops.chtimes(target, before.AccessTime, before.ModifiedTime); err != nil {
			failures = append(failures, fmt.Errorf("restore timestamps: %w", err))
		}
	}
	if changes.permissions {
		if err := ops.chmod(target, before.Permissions); err != nil {
			failures = append(failures, fmt.Errorf("restore permissions: %w", err))
		}
	}
	return errors.Join(failures...)
}

func checkContext(ctx context.Context, op, path string) error {
	if ctx == nil {
		return newError(KindInvalidInput, op, path, errors.New("context must not be nil"))
	}
	if err := ctx.Err(); err != nil {
		return newError(KindCanceled, op, path, err)
	}
	return nil
}
