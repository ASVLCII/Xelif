package dircompare

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CopyMissing copies only regular files and directories that do not exist at
// destination. Existing entries are never overwritten, and destination-only
// entries are never deleted. Type collisions detected during planning abort
// before writes begin; entries appearing later also cause a collision error.
// On failure or cancellation, completed destination entries are retained and
// reported through PartialCopyError; unpublished temporary files are cleaned up.
func CopyMissing(ctx context.Context, sourceRoot, destinationRoot string, options CopyOptions) (CopyResult, error) {
	if err := ctx.Err(); err != nil {
		return CopyResult{}, err
	}
	plan, err := prepareCopy(ctx, sourceRoot, destinationRoot, options)
	if err != nil {
		if options.ApprovedPlan != nil && !errors.Is(err, context.Canceled) &&
			!errors.Is(err, context.DeadlineExceeded) {
			return CopyResult{}, &StalePlanError{Err: err}
		}
		return CopyResult{}, err
	}
	if options.ApprovedPlan != nil {
		if err := plan.validateApproval(ctx, *options.ApprovedPlan); err != nil {
			return CopyResult{}, err
		}
	}
	if options.DryRun {
		approval, err := freezeCopyPlan(ctx, plan)
		if err != nil {
			return CopyResult{}, err
		}
		plan.result.approval = approval
		return plan.result, nil
	}
	executor := copyExecutor{ctx: ctx, options: options, copyPlan: plan}
	return executor.execute()
}

type copyPlan struct {
	source         map[string]treeEntry
	sourceAbs      string
	destinationAbs string
	result         CopyResult
	approval       *copyApproval
}

func prepareCopy(
	ctx context.Context,
	sourceRoot, destinationRoot string,
	options CopyOptions,
) (copyPlan, error) {
	bounds, err := normalizeBounds(options.MaxEntries, options.MaxDepth)
	if err != nil {
		return copyPlan{}, err
	}
	sourceAbs, err := validateRoot(sourceRoot)
	if err != nil {
		return copyPlan{}, err
	}
	destinationAbs, err := validateRoot(destinationRoot)
	if err != nil {
		return copyPlan{}, err
	}
	if rootsOverlap(sourceAbs, destinationAbs) {
		return copyPlan{}, &RootOverlapError{Source: sourceRoot, Destination: destinationRoot}
	}

	state := &traversalState{bounds: bounds}
	source, err := scanCopyTree(ctx, sourceAbs, LeftSide, "scan source", state)
	if err != nil {
		return copyPlan{}, err
	}
	destination, err := scanCopyTree(ctx, destinationAbs, RightSide, "scan destination", state)
	if err != nil {
		return copyPlan{}, err
	}

	paths := sortedTreePaths(source)
	if collisionErr := validateCopyCollisions(paths, source, destination); collisionErr != nil {
		return copyPlan{}, collisionErr
	}
	actions, err := buildCopyActions(paths, source, destination)
	if err != nil {
		return copyPlan{}, err
	}
	return copyPlan{
		source:         source,
		sourceAbs:      sourceAbs,
		destinationAbs: destinationAbs,
		result:         CopyResult{Actions: actions},
	}, nil
}

func scanCopyTree(
	ctx context.Context,
	root string,
	side Side,
	operation string,
	state *traversalState,
) (map[string]treeEntry, error) {
	entries, warnings, err := scanTree(ctx, root, side, state)
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 {
		return nil, &PathError{Op: operation, Path: warnings[0].Path, Err: warnings[0].Err}
	}
	return entries, nil
}

func sortedTreePaths(entries map[string]treeEntry) []string {
	paths := make([]string, 0, len(entries))
	for path := range entries {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func validateCopyCollisions(
	paths []string,
	source, destination map[string]treeEntry,
) error {
	for _, path := range paths {
		destinationEntry, exists := destination[path]
		if exists && source[path].metadata.Type != destinationEntry.metadata.Type {
			return &CollisionError{
				Path:            path,
				SourceType:      source[path].metadata.Type,
				DestinationType: destinationEntry.metadata.Type,
			}
		}
	}
	return nil
}

func buildCopyActions(
	paths []string,
	source, destination map[string]treeEntry,
) ([]CopyAction, error) {
	actions := make([]CopyAction, 0)
	for _, path := range paths {
		entry := source[path]
		if _, exists := destination[path]; exists {
			continue
		}
		action := CopyAction{Path: path, Mode: entry.metadata.Mode.Perm()}
		switch entry.metadata.Type {
		case TypeDirectory:
			action.Kind = CreateDirectory
		case TypeRegular:
			action.Kind = CopyFile
		case TypeSymlink, TypeOther:
			return nil, &UnsupportedTypeError{Path: path, Type: entry.metadata.Type}
		default:
			return nil, &UnsupportedTypeError{Path: path, Type: entry.metadata.Type}
		}
		actions = append(actions, action)
	}
	return actions, nil
}

type copyExecutor struct {
	copyPlan

	ctx     context.Context
	options CopyOptions
}

func (e *copyExecutor) execute() (CopyResult, error) {
	for index, action := range e.result.Actions {
		if err := e.ctx.Err(); err != nil {
			return e.fail(err)
		}
		if err := e.executeAction(action); err != nil {
			return e.fail(err)
		}
		e.result.Created = append(e.result.Created, action.Path)
		e.reportProgress(index, action)
		if err := e.ctx.Err(); err != nil {
			return e.fail(err)
		}
	}
	if err := e.restoreDirectoryModes(); err != nil {
		return e.fail(err)
	}
	return e.result, nil
}

func (e *copyExecutor) executeAction(action CopyAction) error {
	sourceEntry := e.source[action.Path]
	targetPath := e.targetPath(action.Path)
	if err := e.validateApprovedAction(action); err != nil {
		return err
	}
	if err := ensureSafeParents(e.destinationAbs, action.Path); err != nil {
		return err
	}
	if err := ensureTargetAbsent(targetPath, action.Path, sourceEntry.metadata.Type); err != nil {
		return err
	}
	switch action.Kind {
	case CreateDirectory:
		return createDirectory(targetPath, action)
	case CopyFile:
		var expected *sourceFingerprint
		if e.approval != nil {
			fingerprint := e.approval.sources[action.Path]
			expected = &fingerprint
		}
		return copyRegularFile(e.ctx, sourceEntry.absPath, targetPath, action.Path, action.Mode, expected)
	default:
		return &UnsupportedTypeError{Path: action.Path, Type: sourceEntry.metadata.Type}
	}
}

func ensureTargetAbsent(targetPath, displayPath string, sourceType EntryType) error {
	info, err := os.Lstat(targetPath)
	if err == nil {
		return &CollisionError{
			Path:            displayPath,
			SourceType:      sourceType,
			DestinationType: metadataFromInfo(info).Type,
		}
	}
	if !errors.Is(err, os.ErrNotExist) {
		return &PathError{Op: "inspect destination", Path: displayPath, Err: err}
	}
	return nil
}

func createDirectory(targetPath string, action CopyAction) error {
	if err := os.Mkdir(targetPath, action.Mode.Perm()|0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return &CollisionError{
				Path:            action.Path,
				SourceType:      TypeDirectory,
				DestinationType: destinationType(targetPath),
			}
		}
		return &PathError{Op: "create directory", Path: action.Path, Err: err}
	}
	return nil
}

func (e *copyExecutor) restoreDirectoryModes() error {
	// Directories are initially owner-writable so nested entries can be
	// created. Restore their requested modes after all children are complete.
	for index := len(e.result.Actions) - 1; index >= 0; index-- {
		action := e.result.Actions[index]
		if action.Kind != CreateDirectory {
			continue
		}
		if err := e.ctx.Err(); err != nil {
			return err
		}
		targetPath := e.targetPath(action.Path)
		if err := os.Chmod(targetPath, action.Mode.Perm()); err != nil {
			return &PathError{Op: "set directory mode", Path: action.Path, Err: err}
		}
	}
	return nil
}

func (e *copyExecutor) targetPath(displayPath string) string {
	return filepath.Join(e.destinationAbs, filepath.FromSlash(displayPath))
}

func (e *copyExecutor) reportProgress(index int, action CopyAction) {
	if e.options.Progress != nil {
		e.options.Progress(Progress{
			Completed: index + 1,
			Total:     len(e.result.Actions),
			Action:    action,
		})
	}
}

func (e *copyExecutor) fail(cause error) (CopyResult, error) {
	if len(e.result.Created) == 0 {
		return e.result, cause
	}
	// Published names may now refer to replacements or contain external edits.
	// Never delete them, even if their current metadata matches our creation.
	return e.result, &PartialCopyError{
		Created: append([]string(nil), e.result.Created...),
		Err:     cause,
	}
}

func copyRegularFile(
	ctx context.Context,
	sourcePath, targetPath, displayPath string,
	mode os.FileMode,
	expected *sourceFingerprint,
) error {
	source, err := openStableSource(sourcePath, displayPath)
	if err != nil {
		if expected != nil {
			return &StalePlanError{Path: displayPath, Err: err}
		}
		return err
	}
	defer source.Close()
	if verifyErr := validateOpenApprovedSource(source, sourcePath, displayPath, expected); verifyErr != nil {
		return verifyErr
	}

	temporary, err := os.CreateTemp(filepath.Dir(targetPath), tempPrefix)
	if err != nil {
		return &PathError{Op: "create temporary file", Path: displayPath, Err: err}
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()

	if err := copyToTemporary(ctx, temporary, source, displayPath, expected); err != nil {
		return err
	}
	if err := validateOpenApprovedSource(source, sourcePath, displayPath, expected); err != nil {
		return err
	}
	if err := temporary.Chmod(mode.Perm()); err != nil {
		return &PathError{Op: "set file mode", Path: displayPath, Err: err}
	}
	if err := temporary.Sync(); err != nil {
		return &PathError{Op: "sync temporary file", Path: displayPath, Err: err}
	}
	if err := temporary.Close(); err != nil {
		return &PathError{Op: "close temporary file", Path: displayPath, Err: err}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ensureTargetAbsent(targetPath, displayPath, TypeRegular); err != nil {
		return err
	}
	if err := publishTemporary(temporaryPath, targetPath); err != nil {
		if _, statErr := os.Lstat(targetPath); statErr == nil {
			return &CollisionError{
				Path:            displayPath,
				SourceType:      TypeRegular,
				DestinationType: destinationType(targetPath),
			}
		}
		return &PathError{Op: "publish temporary file", Path: displayPath, Err: err}
	}
	return nil
}

func openStableSource(sourcePath, displayPath string) (*os.File, error) {
	before, err := os.Lstat(sourcePath)
	if err != nil {
		return nil, &PathError{Op: "inspect source", Path: displayPath, Err: err}
	}
	if !before.Mode().IsRegular() {
		return nil, &UnsupportedTypeError{Path: displayPath, Type: metadataFromInfo(before).Type}
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return nil, &PathError{Op: "open source", Path: displayPath, Err: err}
	}
	opened, err := source.Stat()
	if err != nil {
		_ = source.Close()
		return nil, &PathError{Op: "inspect open source", Path: displayPath, Err: err}
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		_ = source.Close()
		return nil, &PathError{Op: "open source", Path: displayPath, Err: errors.New("source changed during copy")}
	}
	return source, nil
}

func copyToTemporary(
	ctx context.Context,
	temporary, source *os.File,
	displayPath string,
	expected *sourceFingerprint,
) error {
	var writer io.Writer = temporary
	digest := sha256.New()
	if expected != nil {
		writer = io.MultiWriter(temporary, digest)
	}
	buffer := make([]byte, fileBufferSize)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := source.Read(buffer)
		if n > 0 {
			if _, err := writer.Write(buffer[:n]); err != nil {
				return &PathError{Op: "write temporary file", Path: displayPath, Err: err}
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return &PathError{Op: "read source", Path: displayPath, Err: readErr}
		}
	}
	if expected != nil && !bytes.Equal(digest.Sum(nil), expected.hash[:]) {
		return &StalePlanError{Path: displayPath, Err: errors.New("source content changed since preview")}
	}
	return nil
}

// publishTemporary atomically links a complete same-directory temporary file
// into place. Unlike os.Rename on Unix, Link has create-if-absent semantics and
// therefore cannot replace a target that appears after the final Lstat check.
func publishTemporary(temporaryPath, targetPath string) error {
	if err := os.Link(temporaryPath, targetPath); err != nil {
		return err
	}
	// The target now references the fully written inode. Deferred cleanup in
	// copyRegularFile retries removal if this best-effort unlink fails.
	_ = os.Remove(temporaryPath)
	return nil
}

func ensureSafeParents(root, displayPath string) error {
	parts := strings.Split(displayPath, "/")
	current := root
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, filepath.FromSlash(part))
		info, err := os.Lstat(current)
		if err != nil {
			return &PathError{Op: "inspect destination parent", Path: displayPath, Err: err}
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return &CollisionError{
				Path:            displayPath,
				SourceType:      TypeDirectory,
				DestinationType: metadataFromInfo(info).Type,
			}
		}
	}
	return nil
}

func destinationType(path string) EntryType {
	info, err := os.Lstat(path)
	if err != nil {
		return TypeOther
	}
	return metadataFromInfo(info).Type
}

func rootsOverlap(left, right string) bool {
	return pathWithin(left, right) || pathWithin(right, left)
}

func pathWithin(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}
