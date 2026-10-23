# Second optimization pass — 2026-10-23

Baseline: commit `b9ad18fc17ff5bc2d63d37a17437e34161acfd85`, the already
optimized `perf/resource-and-file-operation-fixes` branch. This pass is developed
on `perf/second-pass-optimization`; the baseline branch is retained.

## Changes

- ANSI image previews write color sequences directly into one sized output
  builder instead of constructing style objects, formatting hex strings, and
  reparsing colors for every half-block cell. A 256-value channel lookup retains
  the old color conversion's rounding, including its one-byte round-down cases.
  Access also respects nonzero image bounds, fixing cropped-image rendering.
- Content searches compile a Unicode lowercase pattern once per query and use
  streaming KMP matching with one reused 32 KiB buffer. They no longer retain
  and lowercase whole files. UTF-8 carry handles split runes and malformed data;
  matching continues through the byte cap after a hit to reject later NUL bytes.
- The Git refresh throttle runs before enumerating and sorting panel entries.
  Ordinary navigation inside the refresh interval now depends on panel count,
  rather than directory size. Forced refreshes and location changes bypass it;
  content changes are included in the next eligible snapshot.
- Git badge lookup indexes exact paths and ancestor aggregate states once per
  snapshot batch instead of scanning all status entries for every file. Exact
  paths, first duplicate entries, directory priority, and per-path errors retain
  the old behavior. The index is local to the batch, avoiding stale cached state.
- File cells already truncated to width skip redundant word wrapping when safe.
  Control characters, malformed UTF-8, transforms, padding, margins, and borders
  retain the original layout path. Unicode layout and styling remain with
  Lipgloss; this does not replace terminal-width handling.

## Measurements against the optimized baseline

Linux/amd64, AMD EPYC 9V74, Go 1.26.10, GOMAXPROCS=4. Five measured pairs follow
one discarded warmup pair, alternating baseline/candidate execution order, with
500 ms windows and no concurrent test/build jobs. Values are medians. Identical
benchmarks run against source overlays that restore only the relevant baseline
production implementation; other optimizations are held constant.

| Workload | Before | After | Speed ratio | Allocated bytes before → after |
| --- | ---: | ---: | ---: | ---: |
| Gradient ANSI encoding, 120×80 pixels | 6.851 ms | 0.305 ms | 22.49× | 3,016,582 → 212,994 |
| Flat ANSI encoding, 120×80 pixels | 3.765 ms | 0.259 ms | 14.55× | 2,116,772 → 212,995 |
| Full ANSI resize/render, 640×480 into 120×40 cells | 8.468 ms | 3.009 ms | 2.81× | 2,891,842 → 594,994 |
| Content search, 1 MiB, absent match | 3.444 ms | 2.394 ms | 1.44× | 6,506,772 → 42,979 |
| Content search, 1 MiB, prefix match | 4.274 ms | 0.080 ms | 53.36× | 7,555,289 → 42,891 |
| Content search, 1 MiB, suffix match | 5.079 ms | 2.171 ms | 2.34× | 7,555,202 → 42,929 |
| Git-enabled synchronous navigation update, 2,000 entries | 57.264 µs | 2.192 µs | 26.12× | 100,992 → 176 |
| Git badge lookup, 10,000 statuses/paths | 2,771.959 ms | 12.596 ms | 220.07× | 1,468,328 → 2,561,252 |
| Scroll + file-panel render, 10,000 entries | 3.140 ms | 2.822 ms | 1.11× | 782,802 → 726,765 |

Gradient encoding uses one allocation instead of 72,108; the full image pipeline
uses 34 instead of 66,171. Content search allocation falls about 99.4% in the
prefix/suffix cases. The prefix search still reads the complete bounded input
for binary detection; the gain comes from avoiding further pattern processing
and whole-file copies. The absent case shows the smaller gain when every byte
still requires pattern processing.

Batch Git lookup trades about 1.1 MB of additional temporary indexing for removal
of quadratic work in the 10,000-entry case. Small batches benefit less: 100-path
lookup measured 0.356 ms → 0.108 ms with 10,971 → 19,417 allocated bytes. The
Git-enabled navigation fixture excludes rendering, execution of returned async
commands, and terminal output. Its large ratio is not whole-screen acceleration.
Git badge lookup excludes invoking the Git subprocess itself.

These are local workload measurements, not application-wide speed, idle power,
retained RSS, or native Windows/macOS results. Full image rendering excludes file
decoding and terminal painting. Small-directory navigation and warm preview-cache
hits will show different gains.

## Regression coverage and reproduction

ANSI output is compared byte-for-byte against the former style-based encoder
over all channel values, image/color models, transparency, odd/empty sizes, and
NO_COLOR. The corrected cropped-image case has a separate expected-output test.
Content matching uses the old whole-string behavior as an oracle, including
1,000 deterministic malformed-byte cases, split matches/runes, byte limits,
late NUL bytes, query reuse, read failures, no-progress readers, and cancellation.
Git tests compare batch and individual lookup semantics and exercise forced,
expired, location-change, new-file, and stale-response refresh paths. Cell tests
compare the old wrapped renderer across styles, controls, Unicode, and widths.

From the repository root, run the corresponding benchmarks with `-run '^$'
and `-benchmem -benchtime=500ms`:

```sh
go test ./src/pkg/file_preview -run '^$' -bench 'Benchmark(ConvertImageToANSI|ANSIRenderer)$' -benchmem
go test ./src/internal/ui/globalsearch -run '^$' -bench '^BenchmarkLocalContentSearch$' -benchmem
go test ./src/internal -run '^$' -bench '^BenchmarkExplorerRootUpdateWithGitStatus$' -benchmem
go test ./src/internal/gitstatus -run '^$' -bench '^BenchmarkSnapshotPanelStates$' -benchmem
go test ./src/internal/ui/filepanel -run '^$' -bench '^BenchmarkFilePanelScrollRender/entries_10000$' -benchmem
```

Raw paired logs, JSON samples/summary, and the measurement runner are retained
under `/workspace/.cache/pass2`; content and Git snapshots/overlays are under
`/workspace/.cache/content-perf` and `/workspace/.cache/git-perf`. The Git batch
baseline adapter invokes the previous independent `StateFor` loop under the new
batch method signature, preserving identical benchmark callers.

## Validation

The full fresh Go suite passed: 35 packages, 2,882 test/subtest passes, seven
existing opt-in/platform skips, and no failures. The affected common, Git status,
content-search, image-preview, and Git refresh tests passed race detection.
`go vet ./...`, the Linux build, all 12 existing terminal integration tests, and
Windows/amd64 plus macOS/arm64 cross-builds passed. Cross-builds establish compile
compatibility; native terminal acceptance remains separate.
The repository's configured golangci-lint v2.12.2 check reports zero issues.
