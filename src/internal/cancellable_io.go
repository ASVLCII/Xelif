package internal

import (
	"context"
	"errors"
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
	return copyFileContextSized(ctx, src, dst, mode, -1, make([]byte, copyBufferSize))
}

const (
	copyBufferSize = 128 * 1024
	smallCopyLimit = 64 * 1024
)

func copyFileContextSized(ctx context.Context, src, dst string, mode os.FileMode, size int64, buffer []byte) error {
	if !mode.IsRegular() {
		return fmt.Errorf("cannot copy mode %s: %w", mode, errors.ErrUnsupported)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()
	if size >= 0 && size <= smallCopyLimit {
		data := buffer[:size+1]
		n, readErr := io.ReadFull(input, data)
		if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
			return publishSmallCopy(ctx, dst, mode, data[:n])
		}
		if readErr != nil {
			return readErr
		}
		// The source grew beyond its snapshot size; stream the complete file.
		if _, seekErr := input.Seek(0, io.SeekStart); seekErr != nil {
			return seekErr
		}
	}
	return publishStreamedCopy(ctx, input, dst, mode, buffer)
}

func publishStreamedCopy(ctx context.Context, input *os.File, dst string, mode os.FileMode, buffer []byte) error {
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
		//nolint:gosec // G703: Inspect only the randomly named staging file created by this copy.
		if info, statErr := os.Lstat(temporary); statErr == nil && os.SameFile(identity, info) {
			//nolint:gosec // G703: Cleanup is restricted to the staging file whose identity still matches.
			_ = os.Remove(temporary)
		}
	}()
	if err := output.Chmod(mode); err != nil {
		return err
	}
	if err := copyWithBuffer(ctx, output, input, buffer); err != nil {
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

func publishSmallCopy(ctx context.Context, destination string, mode os.FileMode, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	//nolint:gosec // G703: A file-manager destination is user-selected; O_EXCL prevents clobbering an existing path.
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	// This bounded write is a safe commit unit. A cancellation during it takes
	// effect between files, preserving the completed file rather than rolling it back.
	n, writeErr := output.Write(data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr != nil {
		identity, statErr := output.Stat()
		_ = output.Close()
		//nolint:gosec // G703: Compare the exclusively created output identity before failure cleanup.
		if current, currentErr := os.Lstat(
			destination,
		); statErr == nil && currentErr == nil &&
			os.SameFile(identity, current) {
			//nolint:gosec // G703: Remove only an incomplete copy whose destination identity still matches.
			_ = os.Remove(destination)
		}
		return writeErr
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close copy; destination retained at %s: %w", destination, err)
	}
	return nil
}

func copyWithBuffer(ctx context.Context, destination io.Writer, source io.Reader, buffer []byte) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := source.Read(buffer)
		if n > 0 {
			written, writeErr := destination.Write(buffer[:n])
			if writeErr != nil {
				return writeErr
			}
			if written != n {
				return io.ErrShortWrite
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
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
	return copyElementWithBuffer(ctx, source, destination, make([]byte, copyBufferSize))
}

func copyElementWithBuffer(ctx context.Context, source, destination string, buffer []byte) error {
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
		return copyFileContextSized(ctx, source, destination, info.Mode(), info.Size(), buffer)
	}
	if !info.IsDir() {
		return fmt.Errorf("unsupported file mode: %s", info.Mode())
	}
	//nolint:gosec // G703: The selected copy destination is unrestricted; Mkdir refuses an existing root.
	if err := os.Mkdir(destination, info.Mode()|0o700); err != nil {
		return err
	}
	entries, readErr := os.ReadDir(source)
	if readErr == nil {
		for _, entry := range entries {
			readErr = copyElementWithBuffer(
				ctx,
				filepath.Join(source, entry.Name()),
				filepath.Join(destination, entry.Name()),
				buffer,
			)
			if readErr != nil {
				break
			}
		}
	}
	// Keep complete entries on failure; do not recursively roll back directories
	// that may now contain externally created or replaced files.
	//nolint:gosec // G703: Restore source permissions on the directory created by this copy.
	modeErr := os.Chmod(destination, info.Mode())
	if readErr != nil {
		return readErr
	}
	return modeErr
}
