//go:build linux

package internal

import (
	"errors"

	"golang.org/x/sys/unix"
)

func renameNoReplace(source, destination string) (bool, error) {
	err := unix.Renameat2(unix.AT_FDCWD, source, unix.AT_FDCWD, destination, unix.RENAME_NOREPLACE)
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) {
		return false, nil
	}
	return true, err
}

func isCrossDeviceRenameError(err error) bool {
	return errors.Is(err, unix.EXDEV)
}
