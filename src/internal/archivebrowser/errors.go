// Package archivebrowser provides read-only browsing and selected extraction
// for ZIP archives.
package archivebrowser

import (
	"errors"
	"fmt"
)

var (
	ErrUnsupportedFormat = errors.New("archivebrowser: unsupported format")
	ErrInvalidArchive    = errors.New("archivebrowser: invalid archive")
	ErrDuplicatePath     = errors.New("archivebrowser: duplicate normalized path")
	ErrUnsafePath        = errors.New("archivebrowser: unsafe path")
	ErrUnsafeType        = errors.New("archivebrowser: unsafe member type")
	ErrNotDirectory      = errors.New("archivebrowser: member is not a directory")
	ErrIsDirectory       = errors.New("archivebrowser: member is a directory")
	ErrLimitExceeded     = errors.New("archivebrowser: byte limit exceeded")
	ErrInvalidLimit      = errors.New("archivebrowser: byte limit must not be negative")
	ErrInvalidArgument   = errors.New("archivebrowser: invalid argument")
	ErrClosed            = errors.New("archivebrowser: archive is closed")
)

// Error adds operation and path context while preserving errors.Is/errors.As.
type Error struct {
	Op          string
	Archive     string
	Member      string
	Destination string
	Err         error
}

func (e *Error) Error() string {
	message := "archivebrowser: " + e.Op
	if e.Archive != "" {
		message += fmt.Sprintf(" archive %q", e.Archive)
	}
	if e.Member != "" {
		message += fmt.Sprintf(" member %q", e.Member)
	}
	if e.Destination != "" {
		message += fmt.Sprintf(" destination %q", e.Destination)
	}
	return message + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error { return e.Err }
