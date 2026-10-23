package xtractr

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// destinationName rejects sibling-prefix matches and platform-specific unsafe
// paths. os.Root then enforces the boundary during path resolution, including
// when a symlink is inserted between validation and opening the file.
func (x *XFile) destinationName(path string) (string, error) {
	base, err := filepath.Abs(x.OutputDir)
	if err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	name, err := filepath.Rel(base, absolute)
	if err != nil || !filepath.IsLocal(name) {
		return "", fmt.Errorf("%w: %s", ErrInvalidPath, path)
	}
	return name, nil
}

func (x *XFile) destinationRoot() (*os.Root, func(), error) {
	if x.outputRoot != nil {
		return x.outputRoot, func() {}, nil
	}
	// Support upstream's directly exported per-format extraction functions too.
	root, err := acquireOutputRoot(x.OutputDir, x.safeDirMode(x.DirMode).Perm())
	if err != nil {
		return nil, func() {}, err
	}
	return root, func() { _ = root.Close() }, nil
}

// acquireOutputRoot rejects a final symlink/junction and verifies that the
// opened directory is the one inspected before opening. Its ancestors are
// user-selected paths and must be trusted while acquiring this initial handle.
func acquireOutputRoot(destination string, mode os.FileMode) (*os.Root, error) {
	destination, err := filepath.Abs(destination)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(destination, mode.Perm()); err != nil {
		return nil, err
	}
	initial, err := os.Lstat(destination)
	if err != nil {
		return nil, err
	}
	if !initial.IsDir() || initial.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: extraction destination is not an ordinary directory", ErrInvalidPath)
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		return nil, err
	}
	if err := verifyOutputRoot(root, initial); err != nil {
		_ = root.Close()
		return nil, err
	}
	return root, nil
}

func verifyOutputRoot(root *os.Root, initial fs.FileInfo) error {
	opened, err := root.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(initial, opened) {
		return fmt.Errorf("%w: extraction destination changed while opening", ErrInvalidPath)
	}
	return nil
}

func (x *XFile) openOutput(path string, flags int, mode os.FileMode) (*os.File, string, error) {
	root, closeRoot, err := x.destinationRoot()
	if err != nil {
		return nil, "", err
	}
	defer closeRoot()
	name, err := x.destinationName(path)
	if err != nil {
		return nil, "", err
	}
	file, err := root.OpenFile(name, flags, mode.Perm())
	if err == nil {
		return file, path, nil
	}
	if !IsErrNameTooLong(err) {
		return nil, "", err
	}
	// Preserve upstream's long-name fallback, using rooted existence checks and
	// opens so truncation cannot reintroduce an unanchored filesystem operation.
	dir, ext := filepath.Dir(name), filepath.Ext(name)
	base := strings.TrimSuffix(filepath.Base(name), ext)
	for attempt := 0; attempt <= 1000; attempt++ {
		suffix := ""
		if attempt > 0 {
			suffix = "~" + strconv.Itoa(attempt)
		}
		candidate := filepath.Join(dir, truncateToBytes(base, max(nameMax-len(ext)-len(suffix), 1))+suffix+ext)
		if _, statErr := root.Lstat(candidate); statErr == nil {
			continue
		} else if !os.IsNotExist(statErr) {
			return nil, "", statErr
		}
		file, err = root.OpenFile(candidate, flags|os.O_EXCL, mode.Perm())
		if os.IsExist(err) {
			continue
		}
		return file, filepath.Join(x.OutputDir, candidate), err
	}
	return nil, "", ErrNameTooLong
}

func (x *XFile) outputChtimes(path string, atime, mtime time.Time) error {
	root, closeRoot, err := x.destinationRoot()
	if err != nil {
		return err
	}
	defer closeRoot()
	name, err := x.destinationName(path)
	if err != nil {
		return err
	}
	return root.Chtimes(name, atime, mtime)
}

func (x *XFile) outputSymlink(target, path string) error {
	root, closeRoot, err := x.destinationRoot()
	if err != nil {
		return err
	}
	defer closeRoot()
	name, err := x.destinationName(path)
	if err != nil {
		return err
	}
	// Root.Symlink permits links pointing outside the root. Reject these before
	// creation, and rely on Root.OpenFile to contain later symlink traversal.
	if filepath.IsAbs(target) || !filepath.IsLocal(filepath.Join(filepath.Dir(name), target)) {
		return fmt.Errorf("%w: symlink %s -> %s", ErrInvalidPath, path, target)
	}
	return root.Symlink(target, name)
}

func (x *XFile) writeOutput(path string, data []byte, mode os.FileMode) error {
	file, _, err := x.openOutput(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
