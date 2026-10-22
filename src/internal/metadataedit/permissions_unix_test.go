//go:build !windows

package metadataedit

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestApplyAndRestorePermissions(t *testing.T) {
	target := writeTestFile(t, "permissions.txt", "data")
	if err := os.Chmod(target, 0o640); err != nil {
		t.Fatal(err)
	}
	mode := fs.FileMode(0o600)
	patch, err := BuildPatch(target, Changes{Permissions: &mode})
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(t.TempDir(), "permissions.json")
	if _, applyErr := Apply(context.Background(), patch, journal); applyErr != nil {
		t.Fatal(applyErr)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions = %04o, want 0600", info.Mode().Perm())
	}
	if _, restoreErr := Restore(context.Background(), journal); restoreErr != nil {
		t.Fatal(restoreErr)
	}
	info, err = os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("restored permissions = %04o, want 0640", info.Mode().Perm())
	}
}

func TestApplyRollsBackPermissionsWhenCanceledBetweenFields(t *testing.T) {
	before := Metadata{
		Path:         "target",
		AccessTime:   time.Now(),
		ModifiedTime: time.Now(),
		Permissions:  0o644,
	}
	after := before
	after.Permissions = 0o600
	after.ModifiedTime = after.ModifiedTime.Add(time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	var modes []fs.FileMode
	ops := fileOps{
		chmod: func(_ string, mode fs.FileMode) error {
			modes = append(modes, mode)
			if len(modes) == 1 {
				cancel()
			}
			return nil
		},
		chtimes: os.Chtimes,
	}
	err := applyChanges(ctx, "target", before, after, changeSet{permissions: true, times: true}, ops)
	if err == nil {
		t.Fatal("applyChanges unexpectedly succeeded")
	}
	if len(modes) != 2 || modes[1] != 0o644 {
		t.Fatalf("chmod calls = %v, want rollback to 0644", modes)
	}
}
