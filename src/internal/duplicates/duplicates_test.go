package duplicates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestScanGroupsIdenticalFilesAndRejectsSameSizeDifferentContent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "z same.txt"), "shared")
	writeFile(t, filepath.Join(root, "nested", "α.txt"), "shared")
	writeFile(t, filepath.Join(root, "different.txt"), "other!")

	result, err := Scan(context.Background(), []string{root}, Options{Workers: 2})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(result.Groups) != 1 {
		t.Fatalf("got %d groups, want 1: %#v", len(result.Groups), result.Groups)
	}
	group := result.Groups[0]
	if group.Size != 6 || group.Digest != "a4d26868017c0ccffe2efe50944ef4211834660cca834c6e9f86dec6a88246fa" {
		t.Fatalf("unexpected group identity: %#v", group)
	}
	wantPaths := absoluteSorted(t, filepath.Join(root, "nested", "α.txt"), filepath.Join(root, "z same.txt"))
	if got := filePaths(group.Files); !reflect.DeepEqual(got, wantPaths) {
		t.Fatalf("group paths = %q, want %q", got, wantPaths)
	}
	for _, file := range group.Files {
		if file.Size != group.Size || file.Digest != group.Digest {
			t.Fatalf("file metadata does not match group: %#v", file)
		}
		if !filepath.IsAbs(file.Path) {
			t.Fatalf("path is not absolute: %q", file.Path)
		}
	}
	if result.FilesScanned != 3 || result.FilesHashed != 3 || result.BytesHashed != 18 {
		t.Fatalf("unexpected counters: %#v", result)
	}
}

func TestScanGroupsEmptyFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "empty 1"), "")
	writeFile(t, filepath.Join(root, "empty 2"), "")
	writeFile(t, filepath.Join(root, "only.txt"), "not empty")

	result, err := Scan(context.Background(), []string{root}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Groups) != 1 || result.Groups[0].Size != 0 {
		t.Fatalf("empty-file groups = %#v", result.Groups)
	}
	if result.Groups[0].Digest != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("empty digest = %q", result.Groups[0].Digest)
	}
	if result.FilesHashed != 2 || result.BytesHashed != 0 {
		t.Fatalf("unexpected counters: %#v", result)
	}
}

func TestScanResultsAreDeterministic(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "d", "4"), "bbbb")
	writeFile(t, filepath.Join(root, "3"), "aa")
	writeFile(t, filepath.Join(root, "2"), "bbbb")
	writeFile(t, filepath.Join(root, "d", "1"), "aa")

	first, err := Scan(context.Background(), []string{root}, Options{Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		got, scanErr := Scan(context.Background(), []string{root}, Options{Workers: 3})
		if scanErr != nil {
			t.Fatal(scanErr)
		}
		if !equalResults(got, first) {
			t.Fatalf("non-deterministic result:\n got %#v\nwant %#v", got, first)
		}
	}
	if len(first.Groups) != 2 || first.Groups[0].Size != 2 || first.Groups[1].Size != 4 {
		t.Fatalf("groups not sorted by size: %#v", first.Groups)
	}
}

func TestScanCountsHardLinksOnceByDefault(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "original")
	link := filepath.Join(root, "hard link")
	writeFile(t, original, "same")
	if err := os.Link(original, link); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	writeFile(t, filepath.Join(root, "copy"), "same")

	result, err := Scan(context.Background(), []string{root}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.FilesScanned != 2 || len(result.Groups) != 1 || len(result.Groups[0].Files) != 2 {
		t.Fatalf("default hard-link result = %#v", result)
	}

	withLinks, err := Scan(context.Background(), []string{root}, Options{IncludeHardLinks: true})
	if err != nil {
		t.Fatal(err)
	}
	if withLinks.FilesScanned != 3 || len(withLinks.Groups) != 1 || len(withLinks.Groups[0].Files) != 3 {
		t.Fatalf("IncludeHardLinks result = %#v", withLinks)
	}
}

func TestScanCountsEachPathOnceAcrossOverlappingRoots(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	writeFile(t, filepath.Join(nested, "one"), "same")
	writeFile(t, filepath.Join(nested, "two"), "same")

	result, err := Scan(context.Background(), []string{root, nested}, Options{IncludeHardLinks: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.FilesScanned != 2 || len(result.Groups) != 1 || len(result.Groups[0].Files) != 2 {
		t.Fatalf("overlapping-root result = %#v", result)
	}
}

func TestScanHonorsFileAndHashBounds(t *testing.T) {
	t.Run("files", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "1"), "x")
		writeFile(t, filepath.Join(root, "2"), "x")
		writeFile(t, filepath.Join(root, "3"), "x")

		result, err := Scan(context.Background(), []string{root}, Options{MaxFiles: 2})
		if err != nil {
			t.Fatal(err)
		}
		if !result.Truncated || result.FilesScanned != 2 || !hasWarning(result.Warnings, OperationLimit) {
			t.Fatalf("file-bound result = %#v", result)
		}
	})

	t.Run("bytes hashed", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "1"), "same")
		writeFile(t, filepath.Join(root, "2"), "same")

		result, err := Scan(context.Background(), []string{root}, Options{MaxBytesHashed: 4})
		if err != nil {
			t.Fatal(err)
		}
		if !result.Truncated || result.BytesHashed != 4 || result.FilesHashed != 1 || len(result.Groups) != 0 {
			t.Fatalf("hash-bound result = %#v", result)
		}
		if !hasWarning(result.Warnings, OperationLimit) {
			t.Fatalf("missing limit warning: %#v", result.Warnings)
		}
	})
}

func TestScanCancellationAndTypedInputErrors(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Scan(canceled, []string{t.TempDir()}, Options{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Scan() error = %v", err)
	}

	tests := []struct {
		name    string
		roots   []string
		options Options
		code    ErrorCode
	}{
		{name: "no roots", code: ErrorInvalidRoots},
		{name: "empty root", roots: []string{""}, code: ErrorInvalidRoot},
		{
			name: "negative files", roots: []string{t.TempDir()},
			options: Options{MaxFiles: -1}, code: ErrorInvalidOptions,
		},
		{
			name: "negative bytes", roots: []string{t.TempDir()},
			options: Options{MaxBytesHashed: -1}, code: ErrorInvalidOptions,
		},
		{
			name: "negative workers", roots: []string{t.TempDir()},
			options: Options{Workers: -1}, code: ErrorInvalidOptions,
		},
		{name: "missing root", roots: []string{filepath.Join(t.TempDir(), "missing")}, code: ErrorInvalidRoot},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Scan(context.Background(), test.roots, test.options)
			var scanErr *ScanError
			if !errors.As(err, &scanErr) || scanErr.Code != test.code {
				t.Fatalf("Scan() error = %#v, want *ScanError code %q", err, test.code)
			}
		})
	}
}

func TestScanSkipsHiddenEntriesUnlessIncluded(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".hidden", "one"), "same")
	writeFile(t, filepath.Join(root, ".hidden", "two"), "same")

	without, err := Scan(context.Background(), []string{root}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if without.FilesScanned != 0 {
		t.Fatalf("hidden files scanned by default: %#v", without)
	}
	with, err := Scan(context.Background(), []string{root}, Options{IncludeHidden: true})
	if err != nil {
		t.Fatal(err)
	}
	if with.FilesScanned != 2 || len(with.Groups) != 1 {
		t.Fatalf("included hidden result = %#v", with)
	}
}

func TestScanDoesNotFollowSymlinksByDefaultAndReportsBrokenTargets(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	writeFile(t, target, "same")
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	without, err := Scan(context.Background(), []string{root}, Options{IncludeHardLinks: true})
	if err != nil {
		t.Fatal(err)
	}
	if without.FilesScanned != 1 {
		t.Fatalf("followed symlink by default: %#v", without)
	}

	broken := filepath.Join(root, "broken")
	if linkErr := os.Symlink(filepath.Join(root, "does-not-exist"), broken); linkErr != nil {
		t.Fatal(linkErr)
	}
	with, err := Scan(context.Background(), []string{root}, Options{FollowSymlinks: true, IncludeHardLinks: true})
	if err != nil {
		t.Fatal(err)
	}
	if with.FilesScanned != 2 || !hasWarning(with.Warnings, OperationStat) {
		t.Fatalf("follow-symlink result = %#v", with)
	}
}

func TestScanAvoidsSymlinkDirectoryCycles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "dir", "one"), "same")
	writeFile(t, filepath.Join(root, "dir", "two"), "same")
	if err := os.Symlink(root, filepath.Join(root, "dir", "cycle")); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}

	done := make(chan struct{})
	var result Result
	var err error
	go func() {
		result, err = Scan(context.Background(), []string{root}, Options{FollowSymlinks: true})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Scan did not terminate at symlink cycle")
	}
	if err != nil || result.FilesScanned != 2 || len(result.Groups) != 1 {
		t.Fatalf("cycle result = %#v, %v", result, err)
	}
}

func TestProgressNeverBlocksScan(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "one"), "same")
	writeFile(t, filepath.Join(root, "two"), "same")
	progress := make(chan Progress)
	done := make(chan error, 1)
	go func() {
		_, err := Scan(context.Background(), []string{root}, Options{Progress: progress})
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Scan blocked on progress receiver")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func absoluteSorted(t *testing.T, paths ...string) []string {
	t.Helper()
	for index, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		paths[index] = filepath.Clean(absolute)
	}
	sort.Strings(paths)
	return paths
}

func filePaths(files []File) []string {
	paths := make([]string, len(files))
	for index, file := range files {
		paths[index] = file.Path
	}
	return paths
}

func equalResults(left, right Result) bool {
	return reflect.DeepEqual(left.Groups, right.Groups) &&
		slices.EqualFunc(left.Warnings, right.Warnings, equalWarnings) &&
		left.FilesScanned == right.FilesScanned &&
		left.FilesHashed == right.FilesHashed &&
		left.BytesHashed == right.BytesHashed &&
		left.Truncated == right.Truncated
}

func equalWarnings(left, right Warning) bool {
	if left.Path != right.Path || left.Operation != right.Operation {
		return false
	}
	if left.Err == nil || right.Err == nil {
		return left.Err == nil && right.Err == nil
	}
	return left.Err.Error() == right.Err.Error()
}

func hasWarning(warnings []Warning, operation Operation) bool {
	for _, warning := range warnings {
		if warning.Operation == operation {
			return true
		}
	}
	return false
}

func TestDefaultWorkerCountIsUsable(t *testing.T) {
	// Keep runtime imported so the test also documents that a zero worker count
	// selects a runtime-derived positive default rather than disabling hashing.
	if runtime.GOMAXPROCS(0) < 1 {
		t.Fatal("invalid runtime worker baseline")
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a"), "x")
	writeFile(t, filepath.Join(root, "b"), "x")
	result, err := Scan(context.Background(), []string{root}, Options{})
	if err != nil || len(result.Groups) != 1 {
		t.Fatalf("default workers result = %#v, %v", result, err)
	}
}

func TestScanDoesNotReportOnePathAsItsOwnDuplicate(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "one")
	writeFile(t, file, "same")
	for _, includeLinks := range []bool{false, true} {
		result, err := Scan(context.Background(), []string{root, file}, Options{IncludeHardLinks: includeLinks})
		if err != nil {
			t.Fatal(err)
		}
		if result.FilesScanned != 1 || result.FilesHashed != 0 || len(result.Groups) != 0 {
			t.Fatalf("IncludeHardLinks=%t counted the overlapping file twice: %#v", includeLinks, result)
		}
	}
}

func TestScanWorkerBufferGrowthPreservesDigests(t *testing.T) {
	root := t.TempDir()
	sizes := []int{0, 1, 31, hashBufferSize - 1, hashBufferSize, hashBufferSize + 1}
	for _, size := range sizes {
		content := strings.Repeat("x", size)
		writeFile(t, filepath.Join(root, fmt.Sprintf("%d-a", size)), content)
		writeFile(t, filepath.Join(root, fmt.Sprintf("%d-b", size)), content)
	}
	result, err := Scan(context.Background(), []string{root}, Options{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Groups) != len(sizes) || result.FilesHashed != 2*len(sizes) {
		t.Fatalf("unexpected groups after buffer growth: %#v", result)
	}
	var total int64
	for i, group := range result.Groups {
		size := sizes[i]
		digest := sha256.Sum256([]byte(strings.Repeat("x", size)))
		if group.Size != int64(size) || group.Digest != hex.EncodeToString(digest[:]) || len(group.Files) != 2 {
			t.Fatalf("incorrect group: %#v", group)
		}
		total += 2 * int64(size)
	}
	if result.BytesHashed != total {
		t.Fatalf("hashed %d bytes, want %d", result.BytesHashed, total)
	}
}
