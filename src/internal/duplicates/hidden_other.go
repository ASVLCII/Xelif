//go:build !windows

package duplicates

import (
	"os"
	"strings"
)

func isHidden(path string, _ os.FileInfo) bool {
	name := filepathBase(path)
	return strings.HasPrefix(name, ".") && name != "." && name != ".."
}

func filepathBase(path string) string {
	for index := len(path) - 1; index >= 0; index-- {
		if os.IsPathSeparator(path[index]) {
			return path[index+1:]
		}
	}
	return path
}
