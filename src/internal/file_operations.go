package internal

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/yorukot/superfile/src/internal/ui/processbar"
	"github.com/yorukot/superfile/src/pkg/utils"
)

// is an equivalent of "cp -P" command
func copyLinkFile(src, dst string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return err
	}
	return os.Symlink(target, dst)
}

func isSymlink(info os.FileInfo) bool {
	return info.Mode()&os.ModeSymlink != 0
}

// copyFile copies a single file
func copyFile(src, dst string, srcInfo os.FileInfo) error {
	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("cannot copy %s with mode %s: %w", src, srcInfo.Mode(), errors.ErrUnsupported)
	}
	//nolint:gosec // G703: This file manager reads an explicitly selected source path; it is not a sandbox root.
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// The earlier duplicate-name check is not an ownership guarantee: another
	// process may create this path before we open it. Never truncate that file.
	//nolint:gosec // G703: The user-selected paste destination is intentionally unrestricted; O_EXCL prevents clobbering.
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}
	if err := dstFile.Close(); err != nil {
		return fmt.Errorf("failed to close destination file: %w", err)
	}
	return nil
}

// pasteDir handles directory copying with progress tracking
func pasteDir(src, dst string, p *processbar.Process, cut bool, processBarModel *processbar.Model) error {
	dst, err := renameIfDuplicate(dst)
	if err != nil {
		return err
	}

	if cut {
		// Let the filesystem decide whether rename is possible. In particular,
		// Unix VolumeName is empty for every mount and cannot detect EXDEV.
		return moveElementNoReplace(src, dst)
	}

	type copiedDirectory struct {
		path string
		mode os.FileMode
	}
	var directories []copiedDirectory
	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		newPath := filepath.Join(dst, relPath)
		if err := actualPasteOperation(info, path, newPath, p, processBarModel); err != nil {
			return err
		}
		if info.IsDir() {
			directories = append(directories, copiedDirectory{path: newPath, mode: info.Mode()})
		}
		return nil
	})
	// Build with owner access even when the source is read-only. Restrict
	// children before parents, after all descendants have been copied.
	for index := len(directories) - 1; index >= 0; index-- {
		directory := directories[index]
		err = errors.Join(err, os.Chmod(directory.path, directory.mode))
	}

	if err != nil {
		return err
	}

	return nil
}

func actualPasteOperation(info os.FileInfo, path string, newPath string,
	p *processbar.Process, processBarModel *processbar.Model) error {
	var err error
	if info.IsDir() {
		// Every parent is created by the walk before its children. Refuse a
		// destination appearing concurrently instead of merging into it.
		return os.Mkdir(newPath, info.Mode()|0o700)
	}
	if isSymlink(info) {
		return copyLinkFile(path, newPath)
	}

	// File
	p.CurrentFile = filepath.Base(path)
	err = copyFile(path, newPath, info)

	if err != nil {
		p.State = processbar.Failed
		pSendErr := processBarModel.SendUpdateProcessMsg(*p, true)
		if pSendErr != nil {
			slog.Error("Error sending process update", "error", pSendErr)
		}
		return err
	}

	p.Done++
	processBarModel.TrySendingUpdateProcessMsg(*p)
	return nil
}

// isAncestor checks if dst is the same as src or a subdirectory of src.
// It handles symlinks by resolving them and applies case-insensitive comparison on Windows.
func isAncestor(src, dst string) bool {
	// Resolve symlinks for both paths
	srcResolved, err := filepath.EvalSymlinks(src)
	if err != nil {
		// If we can't resolve symlinks, fall back to original path
		srcResolved = src
	}

	dstResolved, err := filepath.EvalSymlinks(dst)
	if err != nil {
		// If we can't resolve symlinks, fall back to original path
		dstResolved = dst
	}

	// Get absolute paths. Abs() also Cleans paths to normalize separators and resolve . and ..
	srcAbs, err := filepath.Abs(srcResolved)
	if err != nil {
		return false
	}

	dstAbs, err := filepath.Abs(dstResolved)
	if err != nil {
		return false
	}

	// On Windows, perform case-insensitive comparison
	if runtime.GOOS == utils.OsWindows {
		srcAbs = strings.ToLower(srcAbs)
		dstAbs = strings.ToLower(dstAbs)
	}

	// Check if dst is the same as src
	if srcAbs == dstAbs {
		return true
	}

	// Check if dst is a subdirectory of src
	// Use filepath.Rel to check the relationship
	rel, err := filepath.Rel(srcAbs, dstAbs)
	if err != nil {
		return false
	}

	// Only the parent component escapes the source. Names such as
	// "..nested" are ordinary children and must still reject recursive paste.
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
