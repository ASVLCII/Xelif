package catalog

import "fmt"

// CorruptionError reports malformed or invalid catalog data.
type CorruptionError struct {
	Path string
	Err  error
}

func (e *CorruptionError) Error() string {
	return fmt.Sprintf("catalog %q is corrupt: %v", e.Path, e.Err)
}

func (e *CorruptionError) Unwrap() error { return e.Err }

// VersionError reports a catalog written in an unsupported format version.
type VersionError struct {
	Path      string
	Found     int
	Supported int
}

func (e *VersionError) Error() string {
	return fmt.Sprintf("catalog %q has version %d; supported version is %d", e.Path, e.Found, e.Supported)
}
