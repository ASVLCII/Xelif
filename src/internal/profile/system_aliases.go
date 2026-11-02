package profile

import (
	"os"
	"runtime"
	"strings"
)

// macOS exposes system-owned /var, /tmp and /etc aliases into /private.
// Resolve only these exact known mappings. Arbitrary profile symlinks and
// junctions must still be rejected, including beneath an approved system root.
func canonicalProfilePath(value string) string {
	if runtime.GOOS != "darwin" {
		return value
	}
	return canonicalDarwinRoot(value, os.Readlink)
}

func canonicalDarwinRoot(value string, readlink func(string) (string, error)) string {
	for _, alias := range []string{"/var", "/tmp", "/etc"} {
		if value != alias && !strings.HasPrefix(value, alias+"/") {
			continue
		}
		target, err := readlink(alias)
		if err != nil {
			return value
		}
		if target != "/private"+alias && target != "private"+alias {
			return value
		}
		return "/private" + value
	}
	return value
}
