package dircompare

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

type treeEntry struct {
	metadata Metadata
	absPath  string
}

type traversalState struct {
	count  int
	bounds boundedOptions
}

// Compare reads two directory trees without modifying either one.
func Compare(ctx context.Context, leftRoot, rightRoot string, options Options) (Result, error) {
	bounds, err := normalizeBounds(options.MaxEntries, options.MaxDepth)
	if err != nil {
		return Result{}, err
	}
	state := &traversalState{bounds: bounds}
	left, leftWarnings, err := scanTree(ctx, leftRoot, LeftSide, state)
	if err != nil {
		return Result{Warnings: leftWarnings}, err
	}
	right, rightWarnings, err := scanTree(ctx, rightRoot, RightSide, state)
	warnings := make([]Warning, 0, len(leftWarnings)+len(rightWarnings))
	warnings = append(warnings, leftWarnings...)
	warnings = append(warnings, rightWarnings...)
	if err != nil {
		sortWarnings(warnings)
		return Result{Warnings: warnings}, err
	}

	paths := comparisonPaths(left, right)
	result := Result{Entries: make([]Entry, 0, len(paths)), Warnings: warnings}
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		entry, compareWarnings, compareErr := comparePath(ctx, path, left, right, options.VerifyContent)
		result.Warnings = append(result.Warnings, compareWarnings...)
		if compareErr != nil {
			sortWarnings(result.Warnings)
			return result, compareErr
		}
		result.Entries = append(result.Entries, entry)
	}
	sortWarnings(result.Warnings)
	return result, nil
}

func comparisonPaths(left, right map[string]treeEntry) []string {
	paths := make([]string, 0, len(left)+len(right))
	seen := make(map[string]struct{}, len(left)+len(right))
	for path := range left {
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	for path := range right {
		if _, ok := seen[path]; !ok {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}

func comparePath(
	ctx context.Context,
	path string,
	left, right map[string]treeEntry,
	verifyContent bool,
) (Entry, []Warning, error) {
	leftEntry, leftOK := left[path]
	rightEntry, rightOK := right[path]
	entry := Entry{Path: path}
	if leftOK {
		entry.Left = leftEntry.metadata
	}
	if rightOK {
		entry.Right = rightEntry.metadata
	}
	switch {
	case !rightOK:
		entry.Classification = OnlyLeft
	case !leftOK:
		entry.Classification = OnlyRight
	case leftEntry.metadata.Type != rightEntry.metadata.Type:
		entry.Classification = TypeMismatch
	default:
		same, warnings, err := sameEntry(ctx, path, leftEntry, rightEntry, verifyContent)
		if same {
			entry.Classification = Same
		} else {
			entry.Classification = Different
		}
		return entry, warnings, err
	}
	return entry, nil, nil
}

func scanTree(
	ctx context.Context,
	root string,
	side Side,
	state *traversalState,
) (map[string]treeEntry, []Warning, error) {
	absRoot, err := validateRoot(root)
	if err != nil {
		return nil, nil, err
	}
	scanner := treeScanner{
		ctx:     ctx,
		side:    side,
		state:   state,
		entries: make(map[string]treeEntry),
	}
	err = scanner.scanDirectory(absRoot, "", 0)
	sortWarnings(scanner.warnings)
	return scanner.entries, scanner.warnings, err
}

type treeScanner struct {
	ctx      context.Context
	side     Side
	state    *traversalState
	entries  map[string]treeEntry
	warnings []Warning
}

func (s *treeScanner) scanDirectory(absDir, relDir string, depth int) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	directoryEntries, err := os.ReadDir(absDir)
	if err != nil {
		return s.handleReadDirectoryError(relDir, err)
	}
	for _, directoryEntry := range directoryEntries {
		if err := s.scanEntry(absDir, relDir, directoryEntry.Name(), depth); err != nil {
			return err
		}
	}
	return nil
}

func (s *treeScanner) handleReadDirectoryError(relDir string, err error) error {
	if relDir == "" {
		return &PathError{Op: "read directory", Path: ".", Err: err}
	}
	s.warn(filepath.ToSlash(relDir), err)
	return nil
}

func (s *treeScanner) scanEntry(absDir, relDir, name string, depth int) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	relPath := filepath.Join(relDir, name)
	displayPath := filepath.ToSlash(relPath)
	entryDepth := depth + 1
	if err := s.reserveEntry(displayPath, entryDepth); err != nil {
		return err
	}
	absPath := filepath.Join(absDir, name)
	info, err := os.Lstat(absPath)
	if err != nil {
		s.warn(displayPath, err)
		return nil
	}
	metadata := s.captureMetadata(absPath, displayPath, info)
	s.entries[displayPath] = treeEntry{metadata: metadata, absPath: absPath}
	if metadata.Type == TypeDirectory {
		return s.scanDirectory(absPath, relPath, entryDepth)
	}
	return nil
}

func (s *treeScanner) reserveEntry(path string, depth int) error {
	if depth > s.state.bounds.maxDepth {
		return &LimitExceededError{Option: "MaxDepth", Limit: s.state.bounds.maxDepth, Path: path}
	}
	if s.state.count >= s.state.bounds.maxEntries {
		return &LimitExceededError{Option: "MaxEntries", Limit: s.state.bounds.maxEntries, Path: path}
	}
	s.state.count++
	return nil
}

func (s *treeScanner) captureMetadata(absPath, displayPath string, info os.FileInfo) Metadata {
	metadata := metadataFromInfo(info)
	switch metadata.Type {
	case TypeSymlink:
		target, err := os.Readlink(absPath)
		if err != nil {
			s.warn(displayPath, err)
		} else {
			metadata.LinkTarget = target
		}
	case TypeOther:
		s.warn(displayPath, fmt.Errorf("unsupported filesystem type %s", info.Mode().Type()))
	case TypeRegular, TypeDirectory:
	}
	return metadata
}

func (s *treeScanner) warn(path string, err error) {
	s.warnings = append(s.warnings, Warning{Side: s.side, Path: path, Err: err})
}

func validateRoot(root string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", &RootError{Root: root, Err: err}
	}
	info, err := os.Lstat(absRoot)
	if err != nil {
		return "", &RootError{Root: root, Err: err}
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", &RootError{Root: root, Err: errors.New("symlink roots are not allowed")}
	}
	if !info.IsDir() {
		return "", &RootError{Root: root, Err: errors.New("root is not a directory")}
	}
	return filepath.Clean(absRoot), nil
}

func metadataFromInfo(info os.FileInfo) Metadata {
	metadata := Metadata{
		Exists:  true,
		Size:    info.Size(),
		Mode:    info.Mode(),
		ModTime: info.ModTime(),
	}
	switch {
	case info.Mode().IsRegular():
		metadata.Type = TypeRegular
	case info.IsDir():
		metadata.Type = TypeDirectory
	case info.Mode()&os.ModeSymlink != 0:
		metadata.Type = TypeSymlink
	default:
		metadata.Type = TypeOther
	}
	return metadata
}

func sameEntry(ctx context.Context, path string, left, right treeEntry, verifyContent bool) (bool, []Warning, error) {
	switch left.metadata.Type {
	case TypeDirectory:
		return true, nil, nil
	case TypeSymlink:
		return left.metadata.LinkTarget == right.metadata.LinkTarget, nil, nil
	case TypeRegular:
		if left.metadata.Size != right.metadata.Size {
			return false, nil, nil
		}
		if !verifyContent {
			return left.metadata.ModTime.Equal(right.metadata.ModTime), nil, nil
		}
		leftHash, err := hashFile(ctx, left.absPath)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return false, nil, err
			}
			return false, []Warning{{Side: LeftSide, Path: path, Err: err}}, nil
		}
		rightHash, err := hashFile(ctx, right.absPath)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return false, nil, err
			}
			return false, []Warning{{Side: RightSide, Path: path, Err: err}}, nil
		}
		return leftHash == rightHash, nil, nil
	case TypeOther:
		return left.metadata.Mode.Type() == right.metadata.Mode.Type() &&
			left.metadata.Size == right.metadata.Size, nil, nil
	default:
		return false, nil, fmt.Errorf("unknown entry type %q", left.metadata.Type)
	}
}

func hashFile(ctx context.Context, path string) ([sha256.Size]byte, error) {
	var zero [sha256.Size]byte
	before, err := os.Lstat(path)
	if err != nil {
		return zero, err
	}
	if !before.Mode().IsRegular() {
		return zero, errors.New("refusing to hash a non-regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return zero, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return zero, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return zero, errors.New("file changed before hashing")
	}
	hash := sha256.New()
	buffer := make([]byte, fileBufferSize)
	for {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		n, readErr := file.Read(buffer)
		if n > 0 {
			if _, err := hash.Write(buffer[:n]); err != nil {
				return zero, err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return zero, readErr
		}
	}
	var result [sha256.Size]byte
	copy(result[:], hash.Sum(nil))
	return result, nil
}

func sortWarnings(warnings []Warning) {
	sort.SliceStable(warnings, func(i, j int) bool {
		if warnings[i].Side != warnings[j].Side {
			return warnings[i].Side < warnings[j].Side
		}
		if warnings[i].Path != warnings[j].Path {
			return warnings[i].Path < warnings[j].Path
		}
		return warnings[i].Error() < warnings[j].Error()
	})
}
