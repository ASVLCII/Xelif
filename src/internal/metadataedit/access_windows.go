//go:build windows

package metadataedit

import (
	"errors"
	"io/fs"
	"syscall"
	"time"
)

func fileAccessTime(info fs.FileInfo) (time.Time, error) {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok || data == nil {
		return time.Time{}, errors.New("windows file attributes are unavailable")
	}
	return time.Unix(0, data.LastAccessTime.Nanoseconds()), nil
}
