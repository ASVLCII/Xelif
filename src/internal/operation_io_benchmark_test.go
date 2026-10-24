package internal

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/yorukot/superfile/src/internal/ui/processbar"
)

func BenchmarkOperationFileCopy(b *testing.B) {
	for _, size := range []int{1024, 8 * 1024 * 1024} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			dir := b.TempDir()
			source, destination := filepath.Join(dir, "source"), filepath.Join(dir, "copy")
			if err := os.WriteFile(source, bytes.Repeat([]byte("x"), size), 0o600); err != nil {
				b.Fatal(err)
			}
			info, err := os.Stat(source)
			if err != nil {
				b.Fatal(err)
			}
			processes := processbar.New()
			p := processbar.NewProcess("benchmark", source, processbar.OpCopy, 0)
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if err := actualPasteOperation(info, source, destination, &p, &processes); err != nil {
					b.Fatal(err)
				}
				b.StopTimer()
				if err := os.Remove(destination); err != nil {
					b.Fatal(err)
				}
				b.StartTimer()
			}
		})
	}
}
