package internal

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func BenchmarkStartupRuntimeDiagnostics(b *testing.B) {
	previous := slog.Default()
	b.Cleanup(func() { slog.SetDefault(previous) })
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo})))
	b.ReportAllocs()
	for b.Loop() {
		printRuntimeInfo()
	}
}

func TestRuntimeDiagnosticsPreserveDebugRecords(t *testing.T) {
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	for _, level := range []slog.Level{slog.LevelInfo, slog.LevelDebug} {
		var output bytes.Buffer
		slog.SetDefault(slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: level})))
		printRuntimeInfo()
		if level == slog.LevelInfo {
			if output.Len() != 0 {
				t.Fatalf("runtime diagnostics produced output at info level: %s", output.String())
			}
			continue
		}
		for _, record := range []string{"Runtime information", "Memory usage", "Object sizes", "alloc_bytes=", "model_size_bytes="} {
			if !strings.Contains(output.String(), record) {
				t.Errorf("missing debug record %q: %s", record, output.String())
			}
		}
	}
}
