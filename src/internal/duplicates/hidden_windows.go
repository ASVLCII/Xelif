//go:build windows

package duplicates

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func isHidden(path string, _ os.FileInfo) bool {
	name := filepath.Base(path)
	if strings.HasPrefix(name, ".") && name != "." && name != ".." {
		return true
	}
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attributes, err := syscall.GetFileAttributes(pointer)
	return err == nil && attributes&syscall.FILE_ATTRIBUTE_HIDDEN != 0
}
