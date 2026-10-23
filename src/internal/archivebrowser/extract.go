package archivebrowser

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// ExtractOptions controls selected extraction. The zero value never
// overwrites an existing file.
type ExtractOptions struct {
	Overwrite bool
}

// Extract writes explicitly selected members beneath destination. Selecting a
// directory includes its descendants. At most one options value may be passed.
func (a *Archive) Extract(ctx context.Context, members []string, destination string, options ...ExtractOptions) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.closed {
		return &Error{Op: operationExtract, Archive: a.path, Destination: destination, Err: ErrClosed}
	}
	if ctx == nil {
		return &Error{
			Op:          operationExtract,
			Archive:     a.path,
			Destination: destination,
			Err:         errors.Join(ErrInvalidArgument, errors.New("nil context")),
		}
	}
	if err := ctx.Err(); err != nil {
		return &Error{Op: operationExtract, Archive: a.path, Destination: destination, Err: err}
	}

	opts, err := parseExtractOptions(options)
	if err != nil {
		return &Error{Op: operationExtract, Archive: a.path, Destination: destination, Err: err}
	}
	root, err := inspectDestination(destination)
	if err != nil {
		errorDestination := destination
		if root != "" {
			errorDestination = root
		}
		return &Error{Op: operationExtract, Archive: a.path, Destination: errorDestination, Err: err}
	}
	selected, err := a.selectedNodes(members)
	if err != nil {
		return err
	}
	if err := a.preflightExtraction(ctx, selected, root, opts.Overwrite); err != nil {
		return err
	}
	destinationRoot, err := os.OpenRoot(root)
	if err != nil {
		return &Error{Op: operationExtract, Archive: a.path, Destination: root, Err: err}
	}
	defer destinationRoot.Close()
	// Anchor all writes and rollback to this directory. Path checks alone do not
	// prevent a parent directory from being replaced with a symlink after preflight.
	opened, err := destinationRoot.Stat(".")
	current, currentErr := os.Lstat(root)
	if err != nil || currentErr != nil || current.Mode()&fs.ModeSymlink != 0 || !os.SameFile(opened, current) {
		return &Error{Op: operationExtract, Archive: a.path, Destination: root, Err: ErrUnsafePath}
	}
	return a.extractSelected(ctx, selected, destinationRoot, opts.Overwrite)
}

func parseExtractOptions(options []ExtractOptions) (ExtractOptions, error) {
	if len(options) > 1 {
		return ExtractOptions{}, errors.Join(
			ErrInvalidArgument,
			errors.New("multiple ExtractOptions values"),
		)
	}
	if len(options) == 1 {
		return options[0], nil
	}
	return ExtractOptions{}, nil
}

func inspectDestination(destination string) (string, error) {
	if destination == "" {
		return "", errors.Join(ErrInvalidArgument, errors.New("empty destination"))
	}
	root, err := filepath.Abs(filepath.Clean(destination))
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return root, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return root, ErrUnsafePath
	}
	if !info.IsDir() {
		return root, ErrNotDirectory
	}
	return root, nil
}

func (a *Archive) preflightExtraction(
	ctx context.Context,
	selected []*node,
	root string,
	overwrite bool,
) error {
	for _, n := range selected {
		if err := ctx.Err(); err != nil {
			return &Error{Op: operationExtract, Archive: a.path, Member: n.entry.Path, Destination: root, Err: err}
		}
		modeType := n.entry.Mode.Type()
		if (n.entry.IsDir && modeType != fs.ModeDir) || (!n.entry.IsDir && modeType != 0) {
			return &Error{
				Op:          operationExtract,
				Archive:     a.path,
				Member:      n.entry.Path,
				Destination: root,
				Err:         ErrUnsafeType,
			}
		}
		target, err := extractionTarget(root, n.entry.Path)
		if err != nil {
			return &Error{Op: operationExtract, Archive: a.path, Member: n.entry.Path, Destination: root, Err: err}
		}
		if err := inspectTarget(root, target, n.entry.IsDir, overwrite); err != nil {
			return &Error{Op: operationExtract, Archive: a.path, Member: n.entry.Path, Destination: target, Err: err}
		}
	}
	return nil
}

func (a *Archive) extractSelected(
	ctx context.Context,
	selected []*node,
	root *os.Root,
	overwrite bool,
) error {
	created := make([]string, 0, len(selected))
	for _, n := range selected {
		if err := ctx.Err(); err != nil {
			rollbackExtraction(root, created)
			return &Error{Op: operationExtract, Archive: a.path, Member: n.entry.Path, Destination: root.Name(), Err: err}
		}
		target := filepath.FromSlash(n.entry.Path)
		newPaths, err := a.extractNode(ctx, n, root, target, overwrite)
		created = append(created, newPaths...)
		if err != nil {
			rollbackExtraction(root, created)
			return &Error{Op: operationExtract, Archive: a.path, Member: n.entry.Path, Destination: filepath.Join(root.Name(), target), Err: err}
		}
	}
	return nil
}

func (a *Archive) extractNode(
	ctx context.Context,
	n *node,
	root *os.Root,
	target string,
	overwrite bool,
) ([]string, error) {
	if n.entry.IsDir {
		return makeDirectories(root, target, directoryPerm(n.entry.Mode))
	}
	created, err := makeDirectories(root, filepath.Dir(target), defaultDirectoryPerm)
	if err != nil {
		return created, err
	}
	if err := a.extractFile(ctx, n, root, target, overwrite); err != nil {
		return created, err
	}
	if !overwrite {
		created = append(created, target)
	}
	return created, nil
}

func rollbackExtraction(root *os.Root, created []string) {
	for i := len(created) - 1; i >= 0; i-- {
		_ = root.Remove(created[i])
	}
}

func (a *Archive) selectedNodes(members []string) ([]*node, error) {
	selected := make(map[string]*node)
	for _, member := range members {
		if err := a.selectMember(selected, member); err != nil {
			return nil, err
		}
	}
	return sortSelectedNodes(selected), nil
}

func (a *Archive) selectMember(selected map[string]*node, member string) error {
	normalized, _, err := normalizePath(member, false)
	if err != nil {
		return &Error{Op: "select", Archive: a.path, Member: member, Err: err}
	}
	n := a.nodes[normalized]
	if n == nil {
		return &Error{Op: "select", Archive: a.path, Member: normalized, Err: fs.ErrNotExist}
	}
	selected[normalized] = n
	if !n.entry.IsDir {
		return nil
	}
	prefix := normalized + "/"
	for candidate, child := range a.nodes {
		if strings.HasPrefix(candidate, prefix) {
			selected[candidate] = child
		}
	}
	return nil
}

func sortSelectedNodes(selected map[string]*node) []*node {
	result := make([]*node, 0, len(selected))
	for _, n := range selected {
		result = append(result, n)
	}
	slices.SortFunc(result, compareNodes)
	return result
}

func compareNodes(left, right *node) int {
	leftDepth := strings.Count(left.entry.Path, "/")
	rightDepth := strings.Count(right.entry.Path, "/")
	if leftDepth != rightDepth {
		return leftDepth - rightDepth
	}
	if left.entry.IsDir == right.entry.IsDir {
		return strings.Compare(left.entry.Path, right.entry.Path)
	}
	if left.entry.IsDir {
		return -1
	}
	return 1
}

func extractionTarget(root, member string) (string, error) {
	if runtime.GOOS == "windows" && unsafeWindowsMember(member) {
		return "", ErrUnsafePath
	}
	target := filepath.Join(root, filepath.FromSlash(member))
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return "", ErrUnsafePath
	}
	if relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", ErrUnsafePath
	}
	return target, nil
}

func unsafeWindowsMember(member string) bool {
	for _, component := range strings.Split(member, "/") {
		if strings.Contains(component, ":") || strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") {
			return true
		}
		base := component
		if dot := strings.IndexByte(base, '.'); dot >= 0 {
			base = base[:dot]
		}
		switch strings.ToUpper(base) {
		case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6":
			return true
		case "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6":
			return true
		case "LPT7", "LPT8", "LPT9":
			return true
		}
	}
	return false
}

func inspectTarget(root, target string, directory, overwrite bool) error {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return errors.Join(ErrUnsafePath, err)
	}
	current := root
	parts := strings.Split(relative, string(filepath.Separator))
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return ErrUnsafePath
		}
		last := i == len(parts)-1
		if !last && !info.IsDir() {
			return ErrNotDirectory
		}
		if last {
			return inspectExistingTarget(info, directory, overwrite)
		}
	}
	return nil
}

func inspectExistingTarget(info fs.FileInfo, directory, overwrite bool) error {
	if directory && info.IsDir() {
		return nil
	}
	if !directory && overwrite && info.Mode().IsRegular() {
		return nil
	}
	return fs.ErrExist
}

func makeDirectories(root *os.Root, target string, finalPerm fs.FileMode) ([]string, error) {
	relative := filepath.Clean(target)
	if relative == "." {
		return nil, nil
	}
	parts := strings.Split(relative, string(filepath.Separator))
	current := "."
	created := make([]string, 0, len(parts))
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := root.Lstat(current)
		if err == nil {
			if info.Mode()&fs.ModeSymlink != 0 {
				return created, ErrUnsafePath
			}
			if !info.IsDir() {
				return created, ErrNotDirectory
			}
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return created, err
		}
		perm := defaultDirectoryPerm
		if i == len(parts)-1 && finalPerm != 0 {
			perm = finalPerm
		}
		if err := root.Mkdir(current, perm); err != nil {
			return created, err
		}
		created = append(created, current)
	}
	return created, nil
}

func (a *Archive) extractFile(ctx context.Context, n *node, root *os.Root, target string, overwrite bool) error {
	reader, openErr := n.file.Open()
	if openErr != nil {
		return openErr
	}
	defer reader.Close()

	// Keep the parent pinned for cleanup as well as publication. A failed copy
	// must not remove an unrelated file through a replaced parent path.
	parent, err := root.OpenRoot(filepath.Dir(target))
	if err != nil {
		return err
	}
	defer parent.Close()
	name := filepath.Base(target)
	if !overwrite {
		file, createErr := parent.OpenFile(
			name,
			os.O_WRONLY|os.O_CREATE|os.O_EXCL,
			filePerm(n.entry.Mode),
		)
		if createErr != nil {
			return createErr
		}
		copyErr := copyAndClose(ctx, file, reader)
		if copyErr == nil {
			copyErr = ctx.Err()
		}
		if copyErr != nil {
			_ = parent.Remove(name)
		}
		return copyErr
	}

	temporaryName := ".archivebrowser-" + rand.Text()
	temporary, err := parent.OpenFile(temporaryName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePerm(n.entry.Mode))
	if err != nil {
		return err
	}
	defer parent.Remove(temporaryName)
	if err := temporary.Chmod(filePerm(n.entry.Mode)); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := copyAndClose(ctx, temporary, reader); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return publishExtractedFile(parent, temporaryName, name)
}

// Never delete the old file before renaming: a failed rename must leave it
// intact. If the platform cannot replace the target, return its error without
// removing the user's existing file.
func publishExtractedFile(root *os.Root, temporary, target string) error {
	if info, err := root.Lstat(target); err == nil {
		if !info.Mode().IsRegular() {
			return ErrUnsafePath
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return root.Rename(temporary, target)
}

func copyAndClose(ctx context.Context, destination *os.File, source io.Reader) error {
	_, copyErr := io.Copy(destination, &contextReader{ctx: ctx, reader: source})
	closeErr := destination.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func filePerm(mode fs.FileMode) fs.FileMode {
	if perm := mode.Perm(); perm != 0 {
		return perm
	}
	return defaultFilePerm
}

func directoryPerm(mode fs.FileMode) fs.FileMode {
	if perm := mode.Perm(); perm != 0 {
		return perm
	}
	return defaultDirectoryPerm
}
