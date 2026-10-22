package filepanel

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

// BenchmarkSizeSort keeps fixture creation outside the timer. Each iteration
// sorts the same immutable parent entries, including child-directory reads.
func BenchmarkSizeSort(b *testing.B) {
	for _, directoryCount := range []int{8, 64, 128} {
		b.Run(fmt.Sprintf("directories-%d", directoryCount), func(b *testing.B) {
			root, entries := setupSizeSortBenchmark(b, directoryCount)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				elements := sortFileElement(sortmodel.SortBySize, false, entries, root)
				if len(elements) != directoryCount+16 {
					b.Fatalf("sort returned %d elements, want %d", len(elements), directoryCount+16)
				}
			}
		})
	}
}

func setupSizeSortBenchmark(b *testing.B, directoryCount int) (string, []os.DirEntry) {
	b.Helper()
	root := b.TempDir()
	for directory := range directoryCount {
		path := filepath.Join(root, fmt.Sprintf("directory-%03d", directory))
		if err := os.Mkdir(path, 0700); err != nil {
			b.Fatal(err)
		}
		// Scramble counts independently of name order and include ties.
		for child := range (directory * 7) % 16 {
			childPath := filepath.Join(path, fmt.Sprintf("child-%02d", child))
			if err := os.WriteFile(childPath, nil, 0600); err != nil {
				b.Fatal(err)
			}
		}
	}
	for file := range 16 {
		path := filepath.Join(root, fmt.Sprintf("file-%02d", file))
		if err := os.WriteFile(path, make([]byte, file+1), 0600); err != nil {
			b.Fatal(err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		b.Fatal(err)
	}
	return root, entries
}
