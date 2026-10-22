# Rendering allocation pass — 2026-10-22

Result: lower temporary allocation cost with unchanged checked output. Ordinary
navigation/render timing is approximately unchanged; this is not a claim of a
large whole-application speedup or faster Windows Terminal paint.

## Change and evidence

`MakePrintableWithEscCheck` now returns already-permitted text unchanged. When
cleaning is needed, it copies the accepted prefix once and processes only the
remaining suffix, preserving the original newline/tab-stop state. The existing
sanitization rules are retained, including invalid UTF-8, replacement characters,
Unicode spacing and optional ANSI escape handling. No persistent cache, index,
daemon, refresh-interval or UI change is involved.

The baseline scroll/render allocation profile attributed approximately 41 MiB
of its sampled allocations to this sanitizer, about 4% of the profile. Lipgloss
and ANSI/grapheme processing remain the dominant renderer costs. This pass does
not replace their layout or Unicode handling.

The same generated test fixture was compiled against the original source using
a Go overlay and against the candidate. Both produced sanitizer fingerprint
`755569cd8e4aab5e2a440fce42dfc597848832aa9f745ad056fbe7582b7b8aaf`
over 24,334 inputs. Six original truecolor file-panel fingerprints also match;
the navigation fixture passes against both binaries. These are strong regression
checks, not a proof for every possible string or a native terminal interaction.

An additional 96-frame matrix also matches exactly, twice per version, across
Nerd Font, Unicode, plain mode, `NO_COLOR`, custom light colors and neutral icons;
narrow/wide widths; focused/unfocused and browser/selected states; initial/scrolled
positions; symlinks and absent metadata. Its aggregate fingerprint is
`80baff09cb85a682020b7b10ac7082d854c0a88187ea4d9a2af7c351a852b78c`.
The matrix restores global font/style state and leaves the original six frame
fingerprints unchanged. Its extended test binaries are separate from the timed
binaries, whose recorded hashes are unchanged.

## Paired measurements

Host: Windows/amd64, Intel i5-1135G7, Go 1.26.5, `GOMAXPROCS=2`.
Eight measured old/new pairs follow two warmup pairs, alternating execution order.
Each benchmark uses a 300 ms window. No other project test/build/lint process ran
during comparison. Host background activity and thermal state were not controlled.
All rows below are medians from eight samples per version.

| Workload | Old ns/op | New ns/op | Old B/op | New B/op | Old allocations/op | New allocations/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Root navigation + visible panels, 50 files | 2,034,650 | 1,990,008 | 304,539.5 | 293,379 | 10,813 | 10,553 |
| Root navigation + visible panels, 2,000 files | 1,984,200.5 | 1,976,474.5 | 303,735 | 292,751.5 | 10,637 | 10,377 |
| Scroll + file-panel render, 10,000 entries | 3,116,923 | 3,103,024.5 | 890,169.5 | 852,116 | 28,353 | 28,091.5 |
| Sanitizer, already-valid styled text | 352.95 | 71.205 | 120 | 0 | 4 | 0 |
| Sanitizer, text with tabs | 488.3 | 432.35 | 120 | 96 | 4 | 2 |
| Sanitizer, long prefix + trailing control | 2,106.5 | 803.3 | 1,016 | 384 | 7 | 1 |

The two root navigation fixtures allocate **3.6–3.7% fewer bytes** and
**2.4% fewer allocations**. Their median timing decreases are only 2.19% and
0.39%; host variation is large enough that these are not treated as an established
user-visible speedup. The small sanitizer-only benchmark must not be presented
as application-wide acceleration. Percentages were independently checked with
the deterministic verification tool.

The root benchmark includes synchronous `Update` and visible panel/sidebar
rendering, not execution of returned background commands, the full root `View`
compositor, terminal output or native paint. File-panel rendering uses a fixed
viewport, not rendering all 10,000 entries. Bytes/op describe allocation traffic,
not retained memory, process RSS, idle CPU, power use or search throughput.

## Reproduction and provenance

Local evidence is retained under
`X:\Xelif-work\benchmarks\standout-render-20261022`:

- `Measure-Standout.ps1`: build/verify/compare runner, including benchmark-count
  assertions and balanced ordering. It does not edit production source.
- `common-baseline-overlay.json`, original source snapshot, baseline/candidate
  test binaries and their SHA-256 manifests.
- Raw per-round logs, `samples.csv`, `summary.csv` and `summary.json`.
- Original CPU/allocation profiles and output-fingerprint logs.

The overlay changes only `src/internal/common/string_function.go` to its saved
pre-optimization source. Benchmark code, toolchain and corpus are held constant.
The existing refresh benchmark corpus is reused from
`X:\Xelif-work\benchmarks\refresh-root-update-20261022\corpus`.

This is additional to, and does not reattribute, the earlier
[refresh optimization](refresh-optimization.md). Copy-preview content hashing is
an intentional safety cost and is not included in this rendering speed claim.
