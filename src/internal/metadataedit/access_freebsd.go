//go:build freebsd

package metadataedit

import (
	"errors"
	"io/fs"
	"syscall"
	"time"
)

func fileAccessTime(info fs.FileInfo) (time.Time, error) {
	data, ok := info.Sys().(*syscall.Stat_t)
	if !ok || data == nil {
		return time.Time{}, errors.New("stat data is unavailable")
	}
	return time.Unix(data.Atim.Sec, data.Atim.Nsec), nil
}
