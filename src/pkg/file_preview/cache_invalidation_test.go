package filepreview

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestImagePreviewRefreshesChangedAndDeletedSource(t *testing.T) {
	t.Setenv("TERM", "dumb")
	t.Setenv("TERM_PROGRAM", "")
	path := filepath.Join(t.TempDir(), "image.png")
	writeImage := func(fill color.RGBA, stamp int64) {
		t.Helper()
		img := image.NewRGBA(image.Rect(0, 0, 2, 2))
		for y := range 2 {
			for x := range 2 {
				img.SetRGBA(x, y, fill)
			}
		}
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, time.Unix(stamp, 0), time.Unix(stamp, 0)); err != nil {
			t.Fatal(err)
		}
	}
	p := NewImagePreviewer()
	writeImage(color.RGBA{R: 255, A: 255}, 10)
	first, _, err := p.ImagePreview(path, 2, 2, "#000000", 0)
	if err != nil {
		t.Fatal(err)
	}
	writeImage(color.RGBA{B: 255, A: 255}, 20)
	second, _, err := p.ImagePreview(path, 2, 2, "#000000", 0)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("changed image returned its cached old rendering")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.ImagePreview(path, 2, 2, "#000000", 0); !os.IsNotExist(err) {
		t.Fatalf("deleted image returned cached content: %v", err)
	}
}

type testThumbnailGenerator struct {
	calls atomic.Int32
	run   func(string, string) (string, error)
}

func (*testThumbnailGenerator) supportsExt(string) bool { return true }
func (g *testThumbnailGenerator) generateThumbnail(input, output string) (string, error) {
	g.calls.Add(1)
	return g.run(input, output)
}

func newTestThumbnailCache(t *testing.T, converter *testThumbnailGenerator) *ThumbnailGenerator {
	t.Helper()
	return &ThumbnailGenerator{
		tempDirectory: t.TempDir(), tempFilesCache: make(map[string]thumbnailCacheEntry),
		generators: []thumbnailGeneratorInterface{converter},
	}
}

func TestThumbnailRequestsCoalesceAndRefresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.pdf")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	converter := &testThumbnailGenerator{run: func(input, output string) (string, error) {
		data, err := os.ReadFile(input)
		if err == nil {
			err = os.WriteFile(output, data, 0o600)
		}
		return output, err
	}}
	g := newTestThumbnailCache(t, converter)
	var workers sync.WaitGroup
	results := make(chan string, 16)
	errors := make(chan error, 16)
	for range 16 {
		workers.Go(func() {
			result, err := g.GetThumbnailOrGenerate(path)
			results <- result
			errors <- err
		})
	}
	workers.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var old string
	for result := range results {
		if old != "" && old != result {
			t.Fatal("concurrent requests returned different thumbnails")
		}
		old = result
	}
	if calls := converter.calls.Load(); calls != 1 {
		t.Fatalf("same-source requests launched %d conversions, want 1", calls)
	}
	if err := os.WriteFile(path, []byte("changed source"), 0o600); err != nil {
		t.Fatal(err)
	}
	updated, err := g.GetThumbnailOrGenerate(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(updated)
	if err != nil || string(data) != "changed source" {
		t.Fatalf("stale thumbnail after edit: data=%q err=%v", data, err)
	}
	if calls := converter.calls.Load(); calls != 2 {
		t.Fatalf("source change launched %d total conversions, want 2", calls)
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatalf("previously returned path removed while it may be in use: %v", err)
	}
	if err := g.CleanUp(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("cleanup did not remove generated thumbnails: %v", err)
	}
}

func TestOlderThumbnailConversionDoesNotReplaceNewerRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.pdf")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	converter := &testThumbnailGenerator{run: func(input, output string) (string, error) {
		data, err := os.ReadFile(input)
		if err != nil {
			return "", err
		}
		if string(data) == "old" {
			close(started)
			<-release
		}
		return output, os.WriteFile(output, data, 0o600)
	}}
	g := newTestThumbnailCache(t, converter)
	oldDone := make(chan error, 1)
	go func() {
		_, err := g.GetThumbnailOrGenerate(path)
		oldDone <- err
	}()
	<-started
	if err := os.WriteFile(path, []byte("new revision"), 0o600); err != nil {
		close(release)
		t.Fatal(err)
	}
	newPath, err := g.GetThumbnailOrGenerate(path)
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	if oldErr := <-oldDone; oldErr != nil {
		t.Fatal(oldErr)
	}
	current, err := g.GetThumbnailOrGenerate(path)
	if err != nil || current != newPath || converter.calls.Load() != 2 {
		t.Fatalf("older conversion replaced newer cache: current=%q want=%q calls=%d err=%v",
			current, newPath, converter.calls.Load(), err)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("late conversion removed newer thumbnail: %v", err)
	}
}
