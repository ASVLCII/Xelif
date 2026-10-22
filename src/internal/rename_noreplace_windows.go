//go:build windows

package internal

import (
	"errors"

	"golang.org/x/sys/windows"
)

func renameNoReplace(source, destination string) (bool, error) {
	sourcePtr, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return true, err
	}
	destinationPtr, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return true, err
	}
	// MOVEFILE_REPLACE_EXISTING is deliberately omitted.
	return true, windows.MoveFileEx(sourcePtr, destinationPtr, windows.MOVEFILE_WRITE_THROUGH)
}

func isCrossDeviceRenameError(err error) bool {
	return errors.Is(err, windows.ERROR_NOT_SAME_DEVICE)
}
