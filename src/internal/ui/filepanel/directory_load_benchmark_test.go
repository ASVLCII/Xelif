package filepanel

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Use real directory entries: on Unix, DirEntry.Info performs a fresh stat on
// every call. Fixture creation and config initialization are outside the timer.
func BenchmarkDirectoryLoad(b *testing.B) {
	prepareFilePanelRenderBenchmark(b)
	root := b.TempDir()
	for index := range 2000 {
		name := fmt.Sprintf("record-%05d.txt", index)
		if index == 1000 {
			name = "needle.txt"
		}
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o600); err != nil {
			b.Fatal(err)
		}
	}
	for _, query := range []string{"", "needle"} {
		b.Run("query_"+query, func(b *testing.B) {
			panel := newPanelRefreshFixture(root, true)
			panel.SearchBar.SetValue(query)
			want := 2000
			if query != "" {
				want = 1
			}
			b.ReportAllocs()
			for b.Loop() {
				if elements := panel.getElements(false); len(elements) != want {
					b.Fatalf("got %d elements, want %d", len(elements), want)
				}
			}
		})
	}
}
