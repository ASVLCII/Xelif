package archivebrowser

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPublishFailurePreservesExistingFile(t *testing.T) {
	destination := t.TempDir()
	target := filepath.Join(destination, "keep.txt")
	if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	// A publication failure must never delete the previous user's data.
	if err := publishExtractedFile(root, "missing-temporary", "keep.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("publication error = %v, want missing source", err)
	}
	assertFileContent(t, target, "original")
}

// This context replaces the parent after preflight and temporary-file creation,
// precisely when the first payload read starts. It avoids a probabilistic race.
type swapParentContext struct {
	context.Context

	calls int
	swap  func()
}

func (c *swapParentContext) Err() error {
	c.calls++
	if c.calls == 4 {
		c.swap()
	}
	return nil
}

func TestExtractOverwriteParentSwapCannotModifyOutside(t *testing.T) {
	archive := openArchive(t, writeZIP(t, []zipMember{{name: "nested/keep.txt", body: "replacement"}}))
	destination, outside := t.TempDir(), t.TempDir()
	nested := filepath.Join(destination, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(nested, "keep.txt"), filepath.Join(outside, "keep.txt")} {
		if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Probe privilege before starting extraction so unsupported environments
	// record a skip rather than silently omitting the attempted attack.
	probe := filepath.Join(destination, "symlink-probe")
	if err := makeTestDirectoryLink(outside, probe); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	if err := os.Remove(probe); err != nil {
		t.Fatal(err)
	}
	var attempted, swapped, denied bool
	ctx := &swapParentContext{Context: context.Background(), swap: func() {
		attempted = true
		if err := os.Rename(nested, nested+"-original"); err != nil {
			if runtime.GOOS == "windows" && errors.Is(err, fs.ErrPermission) {
				// Windows holds the rooted parent open without delete sharing,
				// which prevents the attack before path substitution can occur.
				denied = true
				return
			}
			t.Fatal(err)
		}
		if err := makeTestDirectoryLink(outside, nested); err != nil {
			t.Fatal(err)
		}
		swapped = true
	}}
	// A rooted implementation may complete in the originally opened directory
	// or reject the changed path; it must not touch anything outside the root.
	_ = archive.Extract(ctx, []string{"nested/keep.txt"}, destination, ExtractOptions{Overwrite: true})
	if !attempted || (!swapped && !denied) {
		t.Fatal("parent swap was not exercised")
	}
	if denied {
		t.Log("Windows denied parent replacement while the extraction root was open")
	}
	assertFileContent(t, filepath.Join(outside, "keep.txt"), "original")
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 1 {
		t.Fatalf("outside entries = %v, error = %v", entries, err)
	}
}

func makeTestDirectoryLink(target, link string) error {
	if runtime.GOOS == "windows" {
		// Junctions exercise the same reparse-point attack without requiring
		// Windows' optional symbolic-link privilege. Both paths are test fixtures.
		return exec.Command("cmd", "/c", "mklink", "/J", link, target).Run()
	}
	return os.Symlink(target, link)
}

type cancelAfterPayloadContext struct {
	context.Context

	calls int
}

func (c *cancelAfterPayloadContext) Err() error {
	c.calls++
	if c.calls >= 5 {
		return context.Canceled
	}
	return nil
}

func TestExtractCancellationBeforePublishPreservesOriginal(t *testing.T) {
	archive := openArchive(t, writeZIP(t, []zipMember{{name: "keep.txt", body: "replacement"}}))
	destination := t.TempDir()
	target := filepath.Join(destination, "keep.txt")
	if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := &cancelAfterPayloadContext{Context: context.Background()}
	if err := archive.Extract(
		ctx,
		[]string{"keep.txt"},
		destination,
		ExtractOptions{Overwrite: true},
	); !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatalf("extraction error = %v, want cancellation", err)
	}
	assertFileContent(t, target, "original")
	entries, err := os.ReadDir(destination)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary output leaked: %v, %v", entries, err)
	}
}
