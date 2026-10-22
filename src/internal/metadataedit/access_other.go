//go:build !windows && !linux && !darwin && !freebsd

package metadataedit

import (
	"errors"
	"io/fs"
	"time"
)

func fileAccessTime(fs.FileInfo) (time.Time, error) {
	return time.Time{}, errors.New("access-time inspection is unsupported on this platform")
}
