# Behavior-preserving optimization pass (2026-10-22)

This pass changes work performed, not the approved interface, shortcuts, wheel
step, provider order, or refresh timing. Existing dirty project work is retained.
The baseline installed binary was SHA256
`3BE2F8B483F45EE3800837CD4983D884202F0E561EB08D1C0DB0F6DB02ED985D`.

## Measurements

Local Windows/amd64, Go 1.26.5, Intel Core i5-1135G7. Directory sorting uses three
samples at `-benchtime=200ms`; icon calculation uses five at `300ms`. Values below
are medians. Fixture creation is excluded. These are warm local measurements,
not cold disk, network-drive, whole-app startup, or Everything comparisons.

| Workload | Before | After |
| --- | ---: | ---: |
| Size sort: 8 directories + 16 files | 1.541 ms | 0.258 ms |
| Size sort: 64 directories + 16 files | 22.922 ms | 1.902 ms |
| Size sort: 128 directories + 16 files | 56.819 ms | 4.300 ms |
| File-type icon color calculation | 814.2 ns | 268.4 ns |
| Debug-disabled startup diagnostics, one call | 23.505 us | 5.383 ns |

At 128 directories, sort allocations fell from approximately 35,143 to 3,219 per
operation. Directory contents are read at most once per sort instead of once per
comparison. The count cache is discarded after each sort. Only folders compared
against other folders are read, preserving the mixed-pair no-I/O behavior.

The icon calculation precomputes luminance for the fixed palette only. Each call
still evaluates the current theme background, transparency, and fallback. No
extension/path cache or stale-theme cache is added.

Startup diagnostics now return before `ReadMemStats` when debug logging is
disabled. The debug-enabled record names and fields are retained. This removes a
small unnecessary cost; it is not evidence of visibly faster application launch.

## Whole-frame limits

Three-sample frame benchmarks use a 120x40 file panel with 100 or 10,000 synthetic
entries, valid metadata, mixed Unicode/ASCII names, two extra columns, and a
selection. No directory reads occur in the timed render region.

Baseline frame medians were 3.177/3.168 ms; after medians were 3.707/3.540 ms.
Scroll-and-render medians were 3.053/3.166 ms before and 3.199/3.225 ms after.
These short separate batches do not establish a whole-frame improvement or
statistically resolve a regression. No overall scrolling-speed claim is made.
Frame allocations stayed near 0.9 MB and 28,350 allocations. A pure icon-color
microbenchmark win should not be presented as a whole-app speedup.

The separate filename microbenchmark inherited the same `NO_COLOR` environment in
both runs; it is not used to claim a colored whole-frame improvement. The frame
fixture explicitly unsets `NO_COLOR` and asserts truecolor ANSI output.

## Behavior checks and retained boundaries

- Sort regressions compare exact results with the old comparator, including ties,
  reverse order, directories-first grouping, partial-read/error results, and new
  counts on the next sort. Existing equal-size comparator behavior is not changed.
- Color parity covers all 256 grayscale backgrounds, representative chromatic and
  invalid backgrounds, all categories, directories/links, and transparency against
  the original calculation. Six narrow/wide unselected/selected/scrolled frame
  fingerprints match the baseline exactly.
- A cross-build exposed a pre-existing missing Unix `winsize` declaration from
  earlier cleanup. It is restored in the Unix-only file alongside its ioctl caller.
- The live Everything probe reported `everything is not running`; an attempted
  startup of the existing installation did not expose its query window. No native
  latency result or search speedup is claimed. Provider code is unchanged.
- The candidate rendered `Filter files` in Windows Terminal. The live wheel script
  could not proceed because Windows rejected `SetCursorPos`; no renewed live wheel
  pass is claimed. Existing automated interaction tests remain required.
- The first full short-suite run failed an existing compression/extraction wait
  and locked-file cleanup. Test-only completion observers now await terminal
  command messages before extraction and removal; the focused test passed five
  repeats with the same timeout. Archive production code and progress reporting
  are unchanged. The synchronization gaps are verified, but their role in the
  earlier intermittent failure is not conclusively established.

Raw samples are retained locally under `X:\Xelif-work\benchmarks`:
`size-sort-{before,after,red,green}-20261022.txt`,
`icon-{before,after}-20261022.txt`, `render-{before,after}.txt`,
`startup-diagnostics-{before,after}-20261022.txt`, and
`native-search-current-20261022.txt`. The last file records an unavailable-provider
failure, not successful search timing. Reproduction benchmarks are in the relevant
Go test packages. No dependencies were added and nothing was published.

## Final verification

The isolated serialized full short suite and subsequent `go vet ./...` both exited
zero. Full layout sweep passed after the production changes. Final Windows,
Linux/amd64 and macOS/arm64 builds passed; the latter two are cross-compilation,
not native execution. Diff whitespace checking passed. Repository-wide lint still
reports 52 pre-existing findings, with no new findings in this pass's files;
`X:\Xelif-work\lint-optimization-final.json` is the refreshed report. No global
analyzer settings were disabled. The full-suite log is
`X:\Xelif-work\benchmarks\optimization-final-short-suite.txt`.

The 52 lint findings above describe that historical optimization checkpoint. The
subsequent [careful lint cleanup](../lint-cleanup.md) resolved the active lint gate
to zero without globally disabling analyzers.
