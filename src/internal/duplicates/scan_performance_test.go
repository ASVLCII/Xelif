package duplicates

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Distinct sizes avoid hashing, isolating discovery and identity deduplication.
func BenchmarkScanDiscovery(b *testing.B) {
	for _, count := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("files_%d", count), func(b *testing.B) {
			root := createDiscoveryBenchmarkFiles(b, count)
			b.ReportAllocs()
			for b.Loop() {
				result, err := Scan(context.Background(), []string{root}, Options{})
				if err != nil || result.FilesScanned != count || result.FilesHashed != 0 {
					b.Fatalf("scan=%#v error=%v", result, err)
				}
			}
		})
	}
}

func createDiscoveryBenchmarkFiles(b *testing.B, count int) string {
	b.Helper()
	root := b.TempDir()
	for i := range count {
		path := filepath.Join(root, fmt.Sprintf("file-%05d", i))
		file, err := os.Create(path)
		if err != nil {
			b.Fatal(err)
		}
		if err = file.Truncate(int64(i)); err != nil {
			b.Fatal(err)
		}
		if err = file.Close(); err != nil {
			b.Fatal(err)
		}
	}
	return root
}

func BenchmarkScanSmallDuplicates(b *testing.B) {
	for _, size := range []int{0, 32} {
		b.Run(fmt.Sprintf("bytes_%d", size), func(b *testing.B) {
			root := b.TempDir()
			content := make([]byte, size)
			for i := range 1000 {
				if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("file-%05d", i)), content, 0o644); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			for b.Loop() {
				result, err := Scan(context.Background(), []string{root}, Options{Workers: 4})
				if err != nil || result.FilesHashed != 1000 || len(result.Groups) != 1 {
					b.Fatalf("scan=%#v error=%v", result, err)
				}
			}
		})
	}
}
