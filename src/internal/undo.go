package internal

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
)

type pathMove struct {
	from string
	to   string
}

type undoAction struct {
	name  string
	moves []pathMove
}

func (m *model) recordUndo(name string, moves []pathMove) {
	if len(moves) == 0 {
		m.lastUndo = nil
		return
	}

	copiedMoves := append([]pathMove(nil), moves...)
	m.lastUndo = &undoAction{name: name, moves: copiedMoves}
}

func (m *model) undoLastFileOperation() error {
	remaining, err := executeUndo(m.lastUndo)
	m.lastUndo = remaining
	return err
}

func (m *model) getUndoLastFileOperationCmd() tea.Cmd {
	journal := m.lastUndo
	action := cloneUndoAction(journal)
	return func() tea.Msg {
		remaining, err := executeUndo(action)
		return UndoOperationMsg{
			journal:   journal,
			remaining: remaining,
			err:       err,
		}
	}
}

func executeUndo(action *undoAction) (*undoAction, error) {
	remaining := cloneUndoAction(action)
	if remaining == nil {
		return nil, errors.New("nothing to undo")
	}

	for _, move := range remaining.moves {
		if _, err := os.Lstat(move.to); err != nil {
			return remaining, fmt.Errorf("cannot undo %s: moved item is unavailable: %w", remaining.name, err)
		}
		if _, err := os.Lstat(move.from); err == nil {
			if !sameFileIdentity(move.from, move.to) {
				return remaining, fmt.Errorf(
					"cannot undo %s: destination already exists: %s",
					remaining.name,
					move.from,
				)
			}
		} else if !os.IsNotExist(err) {
			return remaining, fmt.Errorf("cannot undo %s: cannot inspect destination: %w", remaining.name, err)
		}
	}

	for i := len(remaining.moves) - 1; i >= 0; i-- {
		move := remaining.moves[i]
		var err error
		if sameFileIdentity(move.from, move.to) {
			err = os.Rename(move.to, move.from)
		} else {
			err = moveElementNoReplace(move.to, move.from)
		}
		if err != nil {
			return remaining, fmt.Errorf("cannot undo %s: %w", remaining.name, err)
		}
		remaining.moves = remaining.moves[:i]
	}

	return nil, nil //nolint:nilnil // Successful undo has no remaining journal; nil is the completed state.
}

func cloneUndoAction(action *undoAction) *undoAction {
	if action == nil {
		return nil
	}
	return &undoAction{
		name:  action.name,
		moves: append([]pathMove(nil), action.moves...),
	}
}

func sameFileIdentity(left, right string) bool {
	leftInfo, leftErr := os.Lstat(left)
	rightInfo, rightErr := os.Lstat(right)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
}

func moveElementNoReplace(source, destination string) error {
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("destination already exists: %s", destination)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect destination: %w", err)
	}

	supported, renameErr := renameNoReplace(source, destination)
	if supported {
		if renameErr == nil {
			return nil
		}
		if !isCrossDeviceRenameError(renameErr) {
			return fmt.Errorf("rename without replacing: %w", renameErr)
		}
	}

	if err := copyElementNoReplace(source, destination); err != nil {
		return err
	}
	if err := os.RemoveAll(source); err != nil {
		// Keep the completed destination copy. Directory removal can fail after
		// partially deleting the source, so deleting the destination here could
		// destroy the only remaining copy of some entries.
		return fmt.Errorf("remove source after safe copy; destination copy was preserved: %w", err)
	}
	return nil
}

func copyElementNoReplace(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("inspect source: %w", err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, readErr := os.Readlink(source)
		if readErr != nil {
			return fmt.Errorf("read source link: %w", readErr)
		}
		if linkErr := os.Symlink(target, destination); linkErr != nil {
			return fmt.Errorf("create destination link without replacing: %w", linkErr)
		}
		return nil
	}

	if info.IsDir() {
		return copyDirectoryNoReplace(source, destination, info.Mode())
	}

	sourceFile, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer sourceFile.Close()

	destinationFile, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode())
	if err != nil {
		return fmt.Errorf("create destination file without replacing: %w", err)
	}
	if _, err = io.Copy(destinationFile, sourceFile); err != nil {
		_ = destinationFile.Close()
		_ = os.Remove(destination)
		return fmt.Errorf("copy source file: %w", err)
	}
	if err = destinationFile.Close(); err != nil {
		_ = os.Remove(destination)
		return fmt.Errorf("close destination file: %w", err)
	}
	return nil
}

func copyDirectoryNoReplace(source, destination string, mode os.FileMode) error {
	if err := os.Mkdir(destination, mode); err != nil {
		return fmt.Errorf("create destination directory without replacing: %w", err)
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		_ = os.RemoveAll(destination)
		return fmt.Errorf("read source directory: %w", err)
	}
	for _, entry := range entries {
		if err := copyElementNoReplace(
			filepath.Join(source, entry.Name()),
			filepath.Join(destination, entry.Name()),
		); err != nil {
			_ = os.RemoveAll(destination)
			return err
		}
	}
	return nil
}
