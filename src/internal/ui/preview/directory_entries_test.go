package preview

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"testing"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui"
)

func TestDirectoryPreviewEntriesMatchFullSort(t *testing.T) {
	directory := t.TempDir()
	// More than one read batch, with directory names interleaved among files
	// and creation order deliberately different from display order.
	for i := 620; i >= 0; i-- {
		name := fmt.Sprintf("entry-%04d-文件", i)
		path := filepath.Join(directory, name)
		if i%5 == 0 {
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	full, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(full, func(i, j int) bool {
		if full[i].IsDir() != full[j].IsDir() {
			return full[i].IsDir()
		}
		return full[i].Name() < full[j].Name()
	})
	for _, limit := range []int{-1, 0, 1, 24, 124, 125, 126, 300, 621, 1000} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			got, err := directoryPreviewEntries(directory, limit)
			if err != nil {
				t.Fatal(err)
			}
			want := full[:min(max(1, limit), len(full))]
			if !slices.EqualFunc(got, want, func(a, b fs.DirEntry) bool {
				return a.Name() == b.Name() && a.Type() == b.Type()
			}) {
				t.Fatalf("bounded entries differ from full directory sort for limit %d", limit)
			}
		})
	}
}

func TestDirectoryPreviewEntriesEmptyAndErrors(t *testing.T) {
	directory := t.TempDir()
	entries, err := directoryPreviewEntries(directory, 20)
	if err != nil || len(entries) != 0 {
		t.Fatalf("empty directory: entries=%v err=%v", entries, err)
	}
	file := filepath.Join(directory, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{file, filepath.Join(directory, "absent")} {
		if _, err := directoryPreviewEntries(path, 20); err == nil {
			t.Fatalf("expected error for %s", path)
		}
	}
}

func TestDirectoryPreviewEntriesDoNotFollowSymlinks(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "z-directory"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(directory, "z-directory"), filepath.Join(directory, "a-link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	entries, err := directoryPreviewEntries(directory, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name() != "z-directory" || entries[1].Type()&os.ModeSymlink == 0 {
		t.Fatalf("symlink classified as a directory: %v", entries)
	}
}

func BenchmarkDirectoryPreview(b *testing.B) {
	if err := common.PopulateGlobalConfigs(); err != nil {
		b.Fatal(err)
	}
	directory := b.TempDir()
	for i := 9999; i >= 0; i-- {
		path := filepath.Join(directory, fmt.Sprintf("entry-%05d", i))
		if i%10 == 0 {
			if err := os.Mkdir(path, 0o700); err != nil {
				b.Fatal(err)
			}
		} else if err := os.WriteFile(path, nil, 0o600); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		frame := renderDirectoryPreview(ui.FilePreviewPanelRenderer(24, 80), directory, 24)
		if frame == "" {
			b.Fatal("empty rendered directory preview")
		}
	}
}
