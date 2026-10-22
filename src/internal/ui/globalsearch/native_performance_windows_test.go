package globalsearch

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// This opt-in probe measures the real native provider, not a fallback or a
// simulated search. Index contents and service state affect these local results.
func BenchmarkEverythingNativeQuery(b *testing.B) {
	if os.Getenv("XELIF_TEST_EVERYTHING") != "1" {
		b.Skip("set XELIF_TEST_EVERYTHING=1 with Everything running and Windows indexed")
	}
	b.ReportAllocs()
	for b.Loop() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		results, err := searchEverythingDirect(ctx, "explorer.exe", 10)
		cancel()
		if err != nil {
			b.Fatal(err)
		}
		if len(results) == 0 || len(results) > 10 {
			b.Fatalf("unexpected result count: %d", len(results))
		}
		for _, result := range results {
			if result.Directory || !strings.Contains(strings.ToLower(result.Path), "explorer.exe") {
				b.Fatalf("unexpected native result: %+v", result)
			}
		}
	}
}
