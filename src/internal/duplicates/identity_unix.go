//go:build unix

package duplicates

import (
	"os"
	"syscall"
)

func identityOf(info os.FileInfo) (fileIdentity, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileIdentity{}, false
	}
	//nolint:unconvert // Stat_t field widths differ between supported Unix targets.
	return fileIdentity{device: uint64(stat.Dev), inode: uint64(stat.Ino)}, true
}
