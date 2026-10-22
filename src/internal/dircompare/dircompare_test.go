package dircompare

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"
)

func TestCompareClassifiesAndSortsEntries(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()
	stamp := time.Unix(1_700_000_000, 0)

	mustMkdir(t, filepath.Join(left, "same dir"), 0o755)
	mustMkdir(t, filepath.Join(right, "same dir"), 0o755)
	mustWrite(t, filepath.Join(left, "alpha.txt"), "left", stamp, 0o644)
	mustWrite(t, filepath.Join(right, "beta.txt"), "right", stamp, 0o644)
	mustWrite(t, filepath.Join(left, "same dir", "same.txt"), "same", stamp, 0o640)
	mustWrite(t, filepath.Join(right, "same dir", "same.txt"), "same", stamp, 0o600)
	mustWrite(t, filepath.Join(left, "different.txt"), "short", stamp, 0o644)
	mustWrite(t, filepath.Join(right, "different.txt"), "longer", stamp, 0o644)
	mustMkdir(t, filepath.Join(left, "mismatch"), 0o755)
	mustWrite(t, filepath.Join(right, "mismatch"), "file", stamp, 0o644)
	mustWrite(t, filepath.Join(left, "unicodé 空.txt"), "ok", stamp, 0o644)
	mustWrite(t, filepath.Join(right, "unicodé 空.txt"), "ok", stamp, 0o644)

	result, err := Compare(context.Background(), left, right, Options{})
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	got := make([]string, 0, len(result.Entries))
	for _, entry := range result.Entries {
		got = append(got, entry.Path+":"+string(entry.Classification))
	}
	want := []string{
		"alpha.txt:only-left",
		"beta.txt:only-right",
		"different.txt:different",
		"mismatch:type-mismatch",
		"same dir:same",
		"same dir/same.txt:same",
		"unicodé 空.txt:same",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("entries = %#v, want %#v", got, want)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("warnings = %v, want none", result.Warnings)
	}
}

func TestCompareVerifyContentHashesEqualSizedFiles(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()
	one := time.Unix(1_700_000_000, 0)
	two := one.Add(time.Hour)
	mustWrite(t, filepath.Join(left, "content.txt"), "same", one, 0o644)
	mustWrite(t, filepath.Join(right, "content.txt"), "same", two, 0o644)
	mustWrite(t, filepath.Join(left, "metadata-lie.txt"), "aaaa", one, 0o644)
	mustWrite(t, filepath.Join(right, "metadata-lie.txt"), "bbbb", one, 0o644)

	fast, err := Compare(context.Background(), left, right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	assertClass(t, fast, "content.txt", Different)
	assertClass(t, fast, "metadata-lie.txt", Same)

	hashed, err := Compare(context.Background(), left, right, Options{VerifyContent: true})
	if err != nil {
		t.Fatal(err)
	}
	assertClass(t, hashed, "content.txt", Same)
	assertClass(t, hashed, "metadata-lie.txt", Different)
}

func TestSameEntryPropagatesHashCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	mustWrite(t, path, "contents", time.Now(), 0o644)
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	entry := treeEntry{metadata: metadataFromInfo(info), absPath: path}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err = sameEntry(ctx, "file", entry, entry, true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestCompareHonorsBoundsAndCancellation(t *testing.T) {
	left := t.TempDir()
	right := t.TempDir()
	mustWrite(t, filepath.Join(left, "one"), "1", time.Now(), 0o644)
	mustWrite(t, filepath.Join(left, "two"), "2", time.Now(), 0o644)

	_, err := Compare(context.Background(), left, right, Options{MaxEntries: 1})
	var limitErr *LimitExceededError
	if !errors.As(err, &limitErr) || limitErr.Option != "MaxEntries" {
		t.Fatalf("error = %v, want MaxEntries LimitExceededError", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Compare(ctx, left, right, Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}

	_, err = Compare(context.Background(), left, right, Options{MaxDepth: -1})
	var optionErr *OptionError
	if !errors.As(err, &optionErr) {
		t.Fatalf("error = %v, want OptionError", err)
	}

	deep := t.TempDir()
	mustMkdir(t, filepath.Join(deep, "level"), 0o755)
	mustWrite(t, filepath.Join(deep, "level", "child"), "x", time.Now(), 0o644)
	_, err = Compare(context.Background(), deep, right, Options{MaxDepth: 1})
	if !errors.As(err, &limitErr) || limitErr.Option != "MaxDepth" {
		t.Fatalf("error = %v, want MaxDepth LimitExceededError", err)
	}
}

func TestCompareDoesNotFollowSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks may require Windows developer mode")
	}
	left := t.TempDir()
	right := t.TempDir()
	outside := t.TempDir()
	mustWrite(t, filepath.Join(outside, "secret"), "secret", time.Now(), 0o644)
	if err := os.Symlink(outside, filepath.Join(left, "link")); err != nil {
		t.Fatal(err)
	}

	result, err := Compare(context.Background(), left, right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 1 || result.Entries[0].Path != "link" || result.Entries[0].Left.Type != TypeSymlink {
		t.Fatalf("entries = %#v, want only the symlink itself", result.Entries)
	}
	for _, entry := range result.Entries {
		if entry.Path == "link/secret" {
			t.Fatal("Compare followed a symlink")
		}
	}
}

func TestHashFileRefusesSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks may require Windows developer mode")
	}
	directory := t.TempDir()
	target := filepath.Join(directory, "target")
	mustWrite(t, target, "secret", time.Now(), 0o644)
	link := filepath.Join(directory, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	_, err := hashFile(context.Background(), link)
	if err == nil {
		t.Fatal("hashFile followed a symlink")
	}
}

func TestCopyMissingDryRunPlansWithoutWriting(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()
	mustMkdir(t, filepath.Join(source, "folder space"), 0o750)
	mustWrite(t, filepath.Join(source, "folder space", "unicodé.txt"), "payload", time.Now(), 0o640)

	result, err := CopyMissing(context.Background(), source, destination, CopyOptions{DryRun: true})
	if err != nil {
		t.Fatalf("CopyMissing() error = %v", err)
	}
	got := make([]string, 0, len(result.Actions))
	for _, action := range result.Actions {
		got = append(got, string(action.Kind)+":"+action.Path)
	}
	want := []string{"create-directory:folder space", "copy-file:folder space/unicodé.txt"}
	if !slices.Equal(got, want) {
		t.Fatalf("actions = %#v, want %#v", got, want)
	}
	if entries, err := os.ReadDir(destination); err != nil || len(entries) != 0 {
		t.Fatalf("destination changed during dry run: entries=%v err=%v", entries, err)
	}
}

func TestCopyMissingDoesNotOverwriteAndRejectsTypeCollision(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()
	mustWrite(t, filepath.Join(source, "present.txt"), "new", time.Now(), 0o644)
	mustWrite(t, filepath.Join(destination, "present.txt"), "old", time.Now(), 0o644)
	mustMkdir(t, filepath.Join(source, "collision"), 0o755)
	mustWrite(t, filepath.Join(destination, "collision"), "keep", time.Now(), 0o644)

	_, err := CopyMissing(context.Background(), source, destination, CopyOptions{})
	var collisionErr *CollisionError
	if !errors.As(err, &collisionErr) || collisionErr.Path != "collision" {
		t.Fatalf("error = %v, want CollisionError for collision", err)
	}
	contents, err := os.ReadFile(filepath.Join(destination, "present.txt"))
	if err != nil || string(contents) != "old" {
		t.Fatalf("existing file changed: contents=%q err=%v", contents, err)
	}
	contents, err = os.ReadFile(filepath.Join(destination, "collision"))
	if err != nil || string(contents) != "keep" {
		t.Fatalf("collision changed: contents=%q err=%v", contents, err)
	}
}

func TestCopyMissingRejectsOverlappingRoots(t *testing.T) {
	source := t.TempDir()
	destination := filepath.Join(source, "nested")
	mustMkdir(t, destination, 0o755)

	_, err := CopyMissing(context.Background(), source, destination, CopyOptions{})
	var overlapErr *RootOverlapError
	if !errors.As(err, &overlapErr) {
		t.Fatalf("error = %v, want RootOverlapError", err)
	}
}

func TestCopyMissingRetainsCopiesWithoutOverwritingLateArrival(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()
	mustWrite(t, filepath.Join(source, "a.txt"), "a", time.Now(), 0o644)
	mustWrite(t, filepath.Join(source, "b.txt"), "source-b", time.Now(), 0o644)

	result, err := CopyMissing(context.Background(), source, destination, CopyOptions{
		Progress: func(progress Progress) {
			if progress.Completed == 1 {
				mustWrite(t, filepath.Join(destination, "b.txt"), "late arrival", time.Now(), 0o600)
			}
		},
	})
	var collisionErr *CollisionError
	if !errors.As(err, &collisionErr) || collisionErr.Path != "b.txt" {
		t.Fatalf("error = %v, want CollisionError for b.txt", err)
	}
	if result.RolledBack {
		t.Fatal("result incorrectly reported rollback")
	}
	assertCopyContents(t, filepath.Join(destination, "a.txt"), "a")
	contents, err := os.ReadFile(filepath.Join(destination, "b.txt"))
	if err != nil || string(contents) != "late arrival" {
		t.Fatalf("late arrival changed: contents=%q err=%v", contents, err)
	}
}

func TestCopyMissingCopiesAtomicallyAndPreservesMode(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()
	mustMkdir(t, filepath.Join(source, "nested"), 0o750)
	mustWrite(t, filepath.Join(source, "nested", "file.txt"), "payload", time.Now(), 0o640)

	result, err := CopyMissing(context.Background(), source, destination, CopyOptions{})
	if err != nil {
		t.Fatalf("CopyMissing() error = %v", err)
	}
	if len(result.Created) != 2 {
		t.Fatalf("created = %#v, want directory and file", result.Created)
	}
	contents, err := os.ReadFile(filepath.Join(destination, "nested", "file.txt"))
	if err != nil || string(contents) != "payload" {
		t.Fatalf("copied contents=%q err=%v", contents, err)
	}
	entries, err := os.ReadDir(filepath.Join(destination, "nested"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if len(entry.Name()) >= len(tempPrefix) && entry.Name()[:len(tempPrefix)] == tempPrefix {
			t.Fatalf("temporary file remained: %q", entry.Name())
		}
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(destination, "nested", "file.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o640 {
			t.Fatalf("file mode = %o, want 640", got)
		}
		info, err = os.Stat(filepath.Join(destination, "nested"))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o750 {
			t.Fatalf("directory mode = %o, want 750", got)
		}
	}
}

func TestPublishTemporaryDoesNotReplaceExisting(t *testing.T) {
	directory := t.TempDir()
	temporary := filepath.Join(directory, "temporary")
	target := filepath.Join(directory, "target")
	mustWrite(t, temporary, "new", time.Now(), 0o600)
	mustWrite(t, target, "old", time.Now(), 0o600)

	if err := publishTemporary(temporary, target); err == nil {
		t.Fatal("publishTemporary replaced an existing target")
	}
	contents, err := os.ReadFile(target)
	if err != nil || string(contents) != "old" {
		t.Fatalf("target changed: contents=%q err=%v", contents, err)
	}
}

func TestCopyRegularFileRejectsTypesCancellationAndExistingTarget(t *testing.T) {
	directory := t.TempDir()
	err := copyRegularFile(
		context.Background(),
		directory,
		filepath.Join(t.TempDir(), "target"),
		"directory",
		0o600,
		nil,
	)
	var unsupportedErr *UnsupportedTypeError
	if !errors.As(err, &unsupportedErr) {
		t.Fatalf("error = %v, want UnsupportedTypeError", err)
	}

	source := filepath.Join(directory, "source")
	mustWrite(t, source, "payload", time.Now(), 0o640)
	destination := t.TempDir()
	target := filepath.Join(destination, "target")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if copyErr := copyRegularFile(ctx, source, target, "target", 0o640, nil); !errors.Is(copyErr, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", copyErr)
	}
	entries, err := os.ReadDir(destination)
	if err != nil || len(entries) != 0 {
		t.Fatalf("canceled copy left entries=%v err=%v", entries, err)
	}

	mustWrite(t, target, "old", time.Now(), 0o600)
	err = copyRegularFile(context.Background(), source, target, "target", 0o640, nil)
	var collisionErr *CollisionError
	if !errors.As(err, &collisionErr) {
		t.Fatalf("error = %v, want CollisionError", err)
	}
	contents, err := os.ReadFile(target)
	if err != nil || string(contents) != "old" {
		t.Fatalf("existing target changed: contents=%q err=%v", contents, err)
	}
}

func TestCopyMissingRetainsCreatedItemsOnCancellation(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()
	mustWrite(t, filepath.Join(source, "a.txt"), "a", time.Now(), 0o644)
	mustWrite(t, filepath.Join(source, "b.txt"), "b", time.Now(), 0o644)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result, err := CopyMissing(ctx, source, destination, CopyOptions{
		Progress: func(progress Progress) {
			if progress.Completed == 1 {
				cancel()
			}
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if result.RolledBack {
		t.Fatal("result incorrectly reported rollback")
	}
	assertCopyContents(t, filepath.Join(destination, "a.txt"), "a")
	assertCopyAbsent(t, filepath.Join(destination, "b.txt"))
}

func TestCopyMissingRetainsFileWhenFinalProgressCancels(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()
	mustWrite(t, filepath.Join(source, "only.txt"), "payload", time.Now(), 0o644)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result, err := CopyMissing(ctx, source, destination, CopyOptions{
		Progress: func(Progress) { cancel() },
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if result.RolledBack {
		t.Fatal("result incorrectly reported rollback")
	}
	assertCopyContents(t, filepath.Join(destination, "only.txt"), "payload")
}

func TestCopyMissingRefusesSymlinkSource(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks may require Windows developer mode")
	}
	source := t.TempDir()
	destination := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	mustWrite(t, outside, "secret", time.Now(), 0o644)
	if err := os.Symlink(outside, filepath.Join(source, "link")); err != nil {
		t.Fatal(err)
	}

	_, err := CopyMissing(context.Background(), source, destination, CopyOptions{})
	var unsupportedErr *UnsupportedTypeError
	if !errors.As(err, &unsupportedErr) || unsupportedErr.Path != "link" {
		t.Fatalf("error = %v, want UnsupportedTypeError for link", err)
	}
	if _, statErr := os.Lstat(filepath.Join(destination, "link")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination link exists or unexpected error: %v", statErr)
	}
}

func TestTypedErrorsAndWarningOrdering(t *testing.T) {
	base := errors.New("base")
	warnings := []Warning{
		{Side: RightSide, Path: "z", Err: base},
		{Side: LeftSide, Path: "b", Err: base},
		{Side: LeftSide, Path: "a", Err: base},
	}
	sortWarnings(warnings)
	got := []string{warnings[0].Path, warnings[1].Path, warnings[2].Path}
	if !slices.Equal(got, []string{"a", "b", "z"}) {
		t.Fatalf("warning order = %#v", got)
	}

	errs := []error{
		Warning{Side: LeftSide, Err: base},
		&OptionError{Option: "MaxEntries", Value: -1, Reason: "negative"},
		&LimitExceededError{Option: "MaxEntries", Limit: 1},
		&LimitExceededError{Option: "MaxDepth", Limit: 1, Path: "nested/file"},
		&RootError{Root: "missing", Err: base},
		&PathError{Op: "read", Path: "file", Err: base},
		&CollisionError{Path: "file", SourceType: TypeRegular, DestinationType: TypeDirectory},
		&UnsupportedTypeError{Path: "link", Type: TypeSymlink},
		&RootOverlapError{Source: "source", Destination: "destination"},
		&RollbackError{Errors: []error{base}},
	}
	for _, err := range errs {
		if err.Error() == "" {
			t.Fatalf("empty Error() for %T", err)
		}
	}
	if !errors.Is(errs[0], base) || !errors.Is(errs[4], base) || !errors.Is(errs[5], base) ||
		!errors.Is(errs[9], base) {
		t.Fatal("typed errors do not unwrap their causes")
	}
	if got := (Warning{Side: RightSide, Path: "file", Err: base}).Error(); got == "" {
		t.Fatal("path warning returned an empty message")
	}

	for _, options := range []Options{
		{MaxEntries: maximumMaxEntries + 1},
		{MaxDepth: maximumMaxDepth + 1},
	} {
		_, err := Compare(context.Background(), t.TempDir(), t.TempDir(), options)
		var optionErr *OptionError
		if !errors.As(err, &optionErr) {
			t.Fatalf("error = %v, want OptionError", err)
		}
	}
}

func TestInvalidRootAndDestinationType(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	_, err := Compare(context.Background(), missing, t.TempDir(), Options{})
	var rootErr *RootError
	if !errors.As(err, &rootErr) || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want RootError wrapping os.ErrNotExist", err)
	}

	file := filepath.Join(t.TempDir(), "file")
	mustWrite(t, file, "x", time.Now(), 0o644)
	_, err = Compare(context.Background(), file, t.TempDir(), Options{})
	if !errors.As(err, &rootErr) {
		t.Fatalf("error = %v, want RootError", err)
	}
	if got := destinationType(file); got != TypeRegular {
		t.Fatalf("destinationType(file) = %q", got)
	}
	if got := destinationType(missing); got != TypeOther {
		t.Fatalf("destinationType(missing) = %q", got)
	}
}

func TestEnsureSafeParentsRejectsMissingAndNonDirectoryParents(t *testing.T) {
	root := t.TempDir()
	err := ensureSafeParents(root, "missing/child")
	var pathErr *PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("error = %v, want PathError", err)
	}

	mustWrite(t, filepath.Join(root, "file"), "x", time.Now(), 0o644)
	err = ensureSafeParents(root, "file/child")
	var collisionErr *CollisionError
	if !errors.As(err, &collisionErr) {
		t.Fatalf("error = %v, want CollisionError", err)
	}
}

func TestCopyMissingValidatesOptionsAndRoots(t *testing.T) {
	_, err := CopyMissing(context.Background(), t.TempDir(), t.TempDir(), CopyOptions{MaxDepth: maximumMaxDepth + 1})
	var optionErr *OptionError
	if !errors.As(err, &optionErr) {
		t.Fatalf("error = %v, want OptionError", err)
	}

	missing := filepath.Join(t.TempDir(), "missing")
	_, err = CopyMissing(context.Background(), missing, t.TempDir(), CopyOptions{})
	var rootErr *RootError
	if !errors.As(err, &rootErr) {
		t.Fatalf("error = %v, want RootError", err)
	}
	_, err = CopyMissing(context.Background(), t.TempDir(), missing, CopyOptions{})
	if !errors.As(err, &rootErr) {
		t.Fatalf("error = %v, want destination RootError", err)
	}
}

func assertClass(t *testing.T, result Result, path string, want Classification) {
	t.Helper()
	for _, entry := range result.Entries {
		if entry.Path == path {
			if entry.Classification != want {
				t.Fatalf("classification for %q = %q, want %q", path, entry.Classification, want)
			}
			return
		}
	}
	t.Fatalf("entry %q not found", path)
}

func mustMkdir(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(path, mode); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, contents string, stamp time.Time, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
}
