package internal

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Wrapping the reader intentionally disables io.Copy's platform fast path:
// cancellation must be checked between bounded chunks, not after a whole file.
type cancellationReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r cancellationReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

func copyFileContext(ctx context.Context, src, dst string, mode os.FileMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.CreateTemp(filepath.Dir(dst), ".xelif-copy-*")
	if err != nil {
		return err
	}
	temporary := output.Name()
	identity, err := output.Stat()
	if err != nil {
		_ = output.Close()
		return err
	}
	defer func() {
		_ = output.Close()
		if info, statErr := os.Lstat(temporary); statErr == nil && os.SameFile(identity, info) {
			_ = os.Remove(temporary)
		}
	}()
	if err := output.Chmod(mode); err != nil {
		return err
	}
	if _, err := io.Copy(output, cancellationReader{ctx: ctx, reader: input}); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if supported, err := renameNoReplace(temporary, dst); supported {
		return err
	}
	// A hard-link publication is atomic and refuses an existing destination.
	if err := os.Link(temporary, dst); err != nil {
		return fmt.Errorf("publish copy without replacing: %w", err)
	}
	return nil
}

func moveElementContext(ctx context.Context, source, destination string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if supported, err := renameNoReplace(source, destination); supported {
		if err == nil {
			return nil
		}
		if !isCrossDeviceRenameError(err) {
			return err
		}
	}
	if err := copyElementContext(ctx, source, destination); err != nil {
		return err
	}
	// Cancellation after copying must leave the source and complete destination.
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.RemoveAll(source)
}

func copyElementContext(ctx context.Context, source, destination string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return copyLinkFile(source, destination)
	}
	if info.Mode().IsRegular() {
		return copyFileContext(ctx, source, destination, info.Mode())
	}
	if !info.IsDir() {
		return fmt.Errorf("unsupported file mode: %s", info.Mode())
	}
	if err := os.Mkdir(destination, info.Mode()|0o700); err != nil {
		return err
	}
	entries, readErr := os.ReadDir(source)
	if readErr == nil {
		for _, entry := range entries {
			readErr = copyElementContext(
				ctx,
				filepath.Join(source, entry.Name()),
				filepath.Join(destination, entry.Name()),
			)
			if readErr != nil {
				break
			}
		}
	}
	// Keep complete entries on failure; do not recursively roll back directories
	// that may now contain externally created or replaced files.
	modeErr := os.Chmod(destination, info.Mode())
	if readErr != nil {
		return readErr
	}
	return modeErr
}
