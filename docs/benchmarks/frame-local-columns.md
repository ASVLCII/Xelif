# Frame-local column rendering — 2026-10-23

File-panel rendering now styles each column separator once per selection state
per frame, instead of once per visible row. Row assembly uses one sized join
instead of repeatedly growing a string builder. The cache is bounded by the
column count and discarded after the frame; it retains no directory entries or
theme state between renders.

The baseline allocation profile attributed about 6.4% of allocation traffic to
separator rendering. Styling/layout in Lipgloss remains the dominant cost.

## Local measurements

Linux/amd64, AMD EPYC 9V74, Go 1.26.10, GOMAXPROCS=4. Six alternating
baseline/candidate pairs, 500 ms per sample, using the existing
`BenchmarkFilePanelScrollRender/entries_10000` fixture. No other build or test
jobs ran during the paired comparison. Values are medians of six samples:

| Measurement per frame | Baseline | Candidate | Reduction |
| --- | ---: | ---: | ---: |
| Time | 3.123 ms | 2.830 ms | 9.4% |
| Allocated bytes | 850,855 | 783,694 | 7.9% |
| Allocations | 28,019 | 26,133 | 6.7% |

This measures scrolling and constructing a fixed visible file panel, including
styling and its border. It does not measure filesystem enumeration, the complete
root view, terminal painting, process RSS, or idle CPU. Host scheduling and
allocation sampling introduce variation; these are local results, not a
universal application speedup.

Run the benchmark from the repository root:

```sh
go test ./src/internal/ui/filepanel -run '^$' \
  -bench '^BenchmarkFilePanelScrollRender/entries_10000$' \
  -benchmem -benchtime=500ms -count=6
```

The paired comparison used test binaries with identical tests and dependencies;
a Go overlay restored only the pre-change `render.go`, `columns.go`, and
`types.go` from commit `734be6ec6ecf691e5592b8711c6dad32105eb11f`
for the baseline. Raw profiles, per-pair output, and the overlay are
retained in `/workspace/.cache/perf` in the development environment.

## Output checks

All six existing full-frame SHA256 fingerprints match the baseline. A new
regression test compares cached output with independently styled rows across
narrow/wide panels, zero to three extra columns, mixed selection, focus, plain
mode, and repeated mutations of the same panel. This checks byte-for-byte output
without replacing the renderer's existing Unicode or ANSI handling.
