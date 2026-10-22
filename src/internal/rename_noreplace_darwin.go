//go:build darwin

package internal

import (
	"errors"

	"golang.org/x/sys/unix"
)

func renameNoReplace(source, destination string) (bool, error) {
	return true, unix.RenamexNp(source, destination, unix.RENAME_EXCL)
}

func isCrossDeviceRenameError(err error) bool {
	return errors.Is(err, unix.EXDEV)
}
