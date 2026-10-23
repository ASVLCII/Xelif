package globalsearch

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

type contentChunkReader struct {
	reader io.Reader
	chunk  int
}

func (r contentChunkReader) Read(buffer []byte) (int, error) {
	return r.reader.Read(buffer[:min(len(buffer), r.chunk)])
}

func contentOracle(data []byte, pattern string, limit int) bool {
	data = data[:min(len(data), limit)]
	return bytes.IndexByte(data, 0) < 0 && strings.Contains(strings.ToLower(string(data)), strings.ToLower(pattern))
}

type contentReaderFunc func([]byte) (int, error)

func (reader contentReaderFunc) Read(buffer []byte) (int, error) { return reader(buffer) }

func TestStreamingContentPropagatesErrorsAndCancellation(t *testing.T) {
	matcher := newContentMatcher("needle")
	readFailure := errors.New("read failed")
	_, readErr := matcher.readerMatches(context.Background(), iotest.ErrReader(readFailure), 100)
	if !errors.Is(readErr, readFailure) {
		t.Fatalf("reader failure lost: %v", readErr)
	}
	noProgress := contentReaderFunc(func([]byte) (int, error) { return 0, nil })
	if _, err := matcher.readerMatches(context.Background(), noProgress, 100); !errors.Is(err, io.ErrNoProgress) {
		t.Fatalf("non-progressing reader did not stop: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	reader := contentReaderFunc(func(buffer []byte) (int, error) {
		cancel()
		return copy(buffer, "needle"), nil
	})
	if found, err := matcher.readerMatches(ctx, reader, 100); found || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation after matching was lost: found=%t err=%v", found, err)
	}
}

func TestStreamingContentMatchesWholeStringOracle(t *testing.T) {
	fixtures := []struct{ data, pattern string }{
		{"plain Needle ending", "needle"},
		{"Greek ΣΟΣ and KELVIN and İ", "kelvin"},
		{"Greek ΣΟΣ and KELVIN and İ", "σοσ"},
		{"Greek ΣΟΣ and KELVIN and İ", "i"},
		{"abABabABac", "ababac"},
		{"needle then \x00 binary", "needle"},
		{"before \xff\xfe after", "\ufffd\ufffd"},
		{"before \xff\xfe after", "\xff"},
		{"abc α ending", "\ufffd"},
		{strings.Repeat("a", int(contentReadBufferBytes)-1) + "KELVIN", "kelvin"},
	}
	for _, fixture := range fixtures {
		for _, chunk := range []int{1, 2, 3, 31, int(contentReadBufferBytes)} {
			for _, limit := range []int{0, 1, 2, 3, len(fixture.data) - 1, len(fixture.data), len(fixture.data) + 3} {
				matcher := newContentMatcher(fixture.pattern)
				reader := contentChunkReader{strings.NewReader(fixture.data), chunk}
				actual, err := matcher.readerMatches(context.Background(), reader, int64(limit))
				want := contentOracle([]byte(fixture.data), fixture.pattern, limit)
				if err != nil || actual != want {
					t.Fatalf(
						"data=%q pattern=%q chunk=%d cap=%d got=%t err=%v want=%t",
						fixture.data, fixture.pattern, chunk, limit, actual, err, want,
					)
				}
			}
		}
	}
}

func TestStreamingContentRandomBytesMatchWholeStringOracle(t *testing.T) {
	random := rand.New(rand.NewPCG(42, 17))
	for index := range 1000 {
		data := make([]byte, random.IntN(200)+1)
		for i := range data {
			data[i] = byte(random.IntN(256))
		}
		start := random.IntN(len(data))
		end := start + random.IntN(len(data)-start) + 1
		pattern := string(data[start:end])
		limit := random.IntN(len(data) + 1)
		matcher := newContentMatcher(pattern)
		reader := contentChunkReader{bytes.NewReader(data), random.IntN(7) + 1}
		actual, err := matcher.readerMatches(context.Background(), reader, int64(limit))
		if err != nil || actual != contentOracle(data, pattern, limit) {
			t.Fatalf("case %d data=%x pattern=%x cap=%d actual=%t err=%v", index, data, pattern, limit, actual, err)
		}
	}
}

func TestStreamingContentReusesPatternAcrossFilesWithoutState(t *testing.T) {
	matcher := newContentMatcher("needle")
	for _, data := range []string{"need", "le", "needle", "not present", "NEEDLE then \x00", "needle"} {
		actual, err := matcher.readerMatches(context.Background(), strings.NewReader(data), defaultMaxContentBytes)
		if err != nil || actual != contentOracle([]byte(data), "needle", len(data)) {
			t.Fatalf("data=%q actual=%t err=%v", data, actual, err)
		}
	}
}

func BenchmarkLocalContentSearch(b *testing.B) {
	for _, position := range []string{"absent", "prefix", "suffix"} {
		b.Run(position, func(b *testing.B) {
			root := b.TempDir()
			data := []byte(strings.Repeat("x", int(defaultMaxContentBytes)))
			want := 0
			if position != "absent" {
				offset := 0
				if position == "suffix" {
					offset = len(data) - len("NeEdLe")
				}
				copy(data[offset:], "NeEdLe")
				want = 1
			}
			if err := os.WriteFile(filepath.Join(root, "document.txt"), data, 0o600); err != nil {
				b.Fatal(err)
			}
			query := Query{Content: "needle"}
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			for b.Loop() {
				result := SearchLocalQuery(context.Background(), root, query, LocalSearchOptions{})
				if result.Err != nil || result.Partial || len(result.Results) != want {
					b.Fatalf("unexpected result: %#v", result)
				}
			}
		})
	}
}
