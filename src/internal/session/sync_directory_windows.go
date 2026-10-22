//go:build windows

package session

func syncDirectory(string) error {
	// Windows does not provide a portable directory fsync operation. The
	// temporary file itself is synced before the atomic replacement.
	return nil
}
