package internal

import (
	"errors"
	"fmt"
	"os"
)

// Keep the entry identity separate from a directory link's destination.
// Snapshots own metadata only; they never retain filesystem handles.
type operationIdentity struct {
	entry     os.FileInfo
	directory os.FileInfo
}

func deleteOperationIdentity(path string, expected map[string]operationIdentity) (operationIdentity, error) {
	if identity, exists := expected[path]; exists {
		return identity, identity.check(path)
	}
	return captureOperationIdentity(path, false)
}

func captureOperationIdentity(path string, directory bool) (operationIdentity, error) {
	entry, err := frozenOperationStat(path, false)
	if err != nil {
		return operationIdentity{}, err
	}
	identity := operationIdentity{entry: entry}
	if directory {
		identity.directory = entry
		if entry.Mode().Type() != os.ModeDir {
			identity.directory, err = frozenOperationStat(path, true)
		}
		if err != nil {
			return operationIdentity{}, err
		}
		if !identity.directory.IsDir() {
			return operationIdentity{}, errors.New("paste destination is not a folder")
		}
	}
	return identity, nil
}

func (identity operationIdentity) check(path string) error {
	if identity.entry == nil {
		return fmt.Errorf("item identity unavailable; refresh and try again: %s", path)
	}
	current, err := frozenOperationStat(path, false)
	if err != nil {
		return fmt.Errorf("item changed or could not be verified; refresh and try again: %s: %w", path, err)
	}
	if !os.SameFile(identity.entry, current) || identity.entry.Mode().Type() != current.Mode().Type() {
		return fmt.Errorf("item changed; refresh and try again: %s", path)
	}
	if identity.directory != nil {
		if current.Mode().Type() != os.ModeDir {
			current, err = frozenOperationStat(path, true)
		}
		if err != nil || !os.SameFile(identity.directory, current) {
			return fmt.Errorf("folder target changed or could not be verified; refresh and try again: %s", path)
		}
	}
	return nil
}

func frozenOperationStat(path string, follow bool) (os.FileInfo, error) {
	stat := os.Lstat
	if follow {
		stat = os.Stat
	}
	info, err := stat(path)
	if err != nil {
		return nil, err
	}
	// As in the undo journal, force Windows' lazy file ID lookup now rather
	// than accidentally loading the replacement's ID during a later check.
	if !os.SameFile(info, info) {
		return nil, errors.New("filesystem identity could not be verified")
	}
	return info, nil
}
