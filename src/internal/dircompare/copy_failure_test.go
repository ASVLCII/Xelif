package dircompare

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// Progress runs synchronously after publication, so these mutations do not
// depend on scheduler timing or sleeps.
func TestCopyMissingRetainsChangedPublishedFileOnCancellation(t *testing.T) {
	for _, replace := range []bool{false, true} {
		name := "modified-in-place"
		if replace {
			name = "replaced"
		}
		t.Run(name, func(t *testing.T) {
			checkChangedPublishedFileCancellation(t, replace)
		})
	}
}

func checkChangedPublishedFileCancellation(t *testing.T, replace bool) {
	t.Helper()
	source, destination := t.TempDir(), t.TempDir()
	mustWrite(t, filepath.Join(source, "a.txt"), "copy", time.Now(), 0o644)
	mustWrite(t, filepath.Join(source, "b.txt"), "later", time.Now(), 0o644)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	original := filepath.Join(t.TempDir(), "original")
	result, err := CopyMissing(ctx, source, destination, CopyOptions{
		Progress: func(progress Progress) {
			if progress.Completed != 1 {
				t.Fatalf("unexpected progress: %+v", progress)
			}
			mutatePublishedCopyFile(t, filepath.Join(destination, progress.Action.Path), original, replace)
			cancel()
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want cancellation", err)
	}
	assertCopyContents(t, filepath.Join(destination, "a.txt"), "edit")
	if replace {
		assertCopyContents(t, original, "copy")
	}
	if !slices.Equal(result.Created, []string{"a.txt"}) || result.RolledBack {
		t.Fatalf("creation audit = %+v, want retained a.txt", result)
	}
	assertCopyAbsent(t, filepath.Join(destination, "b.txt"))
	assertPartialCopyError(t, err, []string{"a.txt"})
}

func mutatePublishedCopyFile(t *testing.T, path, original string, replace bool) {
	t.Helper()
	before, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatal(statErr)
	}
	// Windows loads file IDs lazily for path-based Stat results.
	// Capture this identity before renaming or replacing the path.
	if !os.SameFile(before, before) {
		t.Fatal("could not capture published file identity")
	}
	if replace {
		if renameErr := os.Rename(path, original); renameErr != nil {
			t.Fatal(renameErr)
		}
	}
	// Same length and restored timestamp also defeat metadata-only
	// attempts to decide whether a published path is safe to remove.
	mustWrite(t, path, "edit", before.ModTime(), before.Mode().Perm())
	after, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) ||
		os.SameFile(before, after) == replace {
		t.Fatalf(
			"mutation did not preserve size/time or expected identity: before=%v after=%v",
			before,
			after,
		)
	}
}

func TestCopyMissingRetainsPublishedDirectoryOnCancellation(t *testing.T) {
	for _, scenario := range []string{"empty", "replacement", "external-child"} {
		t.Run(scenario, func(t *testing.T) {
			checkPublishedDirectoryCancellation(t, scenario)
		})
	}
}

func checkPublishedDirectoryCancellation(t *testing.T, scenario string) {
	t.Helper()
	source, destination := t.TempDir(), t.TempDir()
	mustMkdir(t, filepath.Join(source, "a"), 0o755)
	mustWrite(t, filepath.Join(source, "a", "later.txt"), "later", time.Now(), 0o644)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	original := filepath.Join(t.TempDir(), "original")
	result, err := CopyMissing(ctx, source, destination, CopyOptions{
		Progress: func(_ Progress) {
			mutatePublishedCopyDirectory(t, filepath.Join(destination, "a"), original, scenario)
			cancel()
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want cancellation", err)
	}
	info, statErr := os.Stat(filepath.Join(destination, "a"))
	if statErr != nil || !info.IsDir() {
		t.Fatalf("published directory lost: info=%v err=%v", info, statErr)
	}
	if !slices.Equal(result.Created, []string{"a"}) || result.RolledBack {
		t.Fatalf("creation audit = %+v, want retained directory", result)
	}
	if scenario == "external-child" {
		assertCopyContents(t, filepath.Join(destination, "a", "external.txt"), "keep")
	}
	if scenario == "replacement" {
		info, statErr := os.Stat(original)
		if statErr != nil || !info.IsDir() {
			t.Fatalf("renamed original directory lost: info=%v err=%v", info, statErr)
		}
		entries, readErr := os.ReadDir(original)
		if readErr != nil || len(entries) != 0 {
			t.Fatalf("renamed original directory changed: entries=%v err=%v", entries, readErr)
		}
	}
	assertCopyAbsent(t, filepath.Join(destination, "a", "later.txt"))
	assertPartialCopyError(t, err, []string{"a"})
}

func mutatePublishedCopyDirectory(t *testing.T, path, original, scenario string) {
	t.Helper()
	if scenario == "replacement" {
		if renameErr := os.Rename(path, original); renameErr != nil {
			t.Fatal(renameErr)
		}
		mustMkdir(t, path, 0o755)
	}
	if scenario == "external-child" {
		mustWrite(t, filepath.Join(path, "external.txt"), "keep", time.Now(), 0o644)
	}
}

func TestCopyMissingRetainsCompletedFileAfterLateCollision(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	mustWrite(t, filepath.Join(source, "a.txt"), "completed", time.Now(), 0o644)
	mustWrite(t, filepath.Join(source, "b.txt"), "source", time.Now(), 0o644)
	result, err := CopyMissing(context.Background(), source, destination, CopyOptions{
		Progress: func(progress Progress) {
			mustWrite(t, filepath.Join(destination, "b.txt"), "external", time.Now(), 0o644)
		},
	})
	var collision *CollisionError
	if !errors.As(err, &collision) || collision.Path != "b.txt" {
		t.Fatalf("error = %v, want late collision at b.txt", err)
	}
	assertCopyContents(t, filepath.Join(destination, "a.txt"), "completed")
	assertCopyContents(t, filepath.Join(destination, "b.txt"), "external")
	if !slices.Equal(result.Created, []string{"a.txt"}) || result.RolledBack {
		t.Fatalf("creation audit = %+v, want retained a.txt", result)
	}
	assertPartialCopyError(t, err, []string{"a.txt"})
}

func TestCopyMissingCancellationBeforePublicationLeavesDestinationEmpty(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	mustWrite(t, filepath.Join(source, "a.txt"), "source", time.Now(), 0o644)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := CopyMissing(ctx, source, destination, CopyOptions{})
	if !errors.Is(err, context.Canceled) || len(result.Created) != 0 || result.RolledBack {
		t.Fatalf("result=%+v err=%v, want cancellation without creations", result, err)
	}
	var partial *PartialCopyError
	if errors.As(err, &partial) {
		t.Fatalf("error = %v, want raw cancellation before any publication", err)
	}
	entries, readErr := os.ReadDir(destination)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("destination entries=%v err=%v, want empty", entries, readErr)
	}
}

func TestPartialCopyErrorSnapshotsFinalPublicationAudit(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	mustMkdir(t, filepath.Join(source, "nested"), 0o755)
	mustWrite(t, filepath.Join(source, "nested", "only.txt"), "complete", time.Now(), 0o644)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := CopyMissing(ctx, source, destination, CopyOptions{
		Progress: func(progress Progress) {
			if progress.Completed == progress.Total {
				cancel()
			}
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want final-publication cancellation", err)
	}
	want := []string{"nested", "nested/only.txt"}
	partial := assertPartialCopyError(t, err, want)
	if !slices.Equal(result.Created, want) || result.RolledBack {
		t.Fatalf("creation audit = %+v, want retained final file", result)
	}
	assertCopyContents(t, filepath.Join(destination, "nested", "only.txt"), "complete")
	result.Created[0] = "caller-mutated-result"
	if !slices.Equal(partial.Created, want) {
		t.Fatalf("error audit changed with result: %v", partial.Created)
	}
	partial.Created[1] = "caller-mutated-error"
	if result.Created[1] != want[1] {
		t.Fatalf("result audit changed with error: %v", result.Created)
	}
}

func TestCopyExecutorCancellationBeforeFirstActionReturnsRawCause(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	mustWrite(t, filepath.Join(source, "only.txt"), "source", time.Now(), 0o644)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	plan, err := prepareCopy(ctx, source, destination, CopyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	executor := copyExecutor{ctx: ctx, copyPlan: plan}
	result, err := executor.execute()
	if !errors.Is(err, context.Canceled) || len(result.Created) != 0 || result.RolledBack {
		t.Fatalf("result=%+v err=%v, want raw cancellation without creations", result, err)
	}
	assertCopyAbsent(t, filepath.Join(destination, "only.txt"))
}

func TestPartialCopyErrorMessageDoesNotListCreatedPaths(t *testing.T) {
	partial := &PartialCopyError{Created: []string{strings.Repeat("long-relative-path/", 1000)}, Err: context.Canceled}
	if len(partial.Error()) > 200 || strings.Contains(partial.Error(), "long-relative-path") {
		t.Fatalf("partial copy message is not bounded: %q", partial.Error())
	}
	if !errors.Is(partial, context.Canceled) {
		t.Fatal("partial copy error does not unwrap cancellation")
	}
}

func assertPartialCopyError(t *testing.T, err error, created []string) *PartialCopyError {
	t.Helper()
	var partial *PartialCopyError
	if !errors.As(err, &partial) || !slices.Equal(partial.Created, created) {
		t.Fatalf("error=%v audit=%+v, want PartialCopyError with %v", err, partial, created)
	}
	if !strings.Contains(partial.Error(), "completed destination entries retained") {
		t.Fatalf("error does not describe retention: %v", partial)
	}
	for _, path := range partial.Created {
		if filepath.IsAbs(path) || strings.Contains(path, `\`) {
			t.Fatalf("created path %q is not slash-separated and relative", path)
		}
	}
	return partial
}

func assertCopyContents(t *testing.T, path, want string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != want {
		t.Fatalf("%s contents=%q err=%v, want %q", path, contents, err, want)
	}
}

func assertCopyAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s stat=%v, want absent", path, err)
	}
}
