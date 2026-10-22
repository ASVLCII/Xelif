//go:build windows

package metadataedit

import (
	"errors"
	"io/fs"
	"testing"
)

func TestPermissionPatchUnsupportedOnWindows(t *testing.T) {
	mode := fs.FileMode(0o600)
	_, err := BuildPatch("file", Changes{Permissions: &mode})
	assertKind(t, err, KindUnsupported)
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("error = %v, want errors.Is ErrUnsupported", err)
	}
}
