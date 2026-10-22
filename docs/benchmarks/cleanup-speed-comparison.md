# Paired speed comparison: before and after lint cleanup

Date: 2026-10-22. This comparison is about the lint cleanup, not original upstream
superfile versus Xelif, and not the earlier folder-size sorting optimization.

## Result

Full-frame medians are practically similar. Two isolated dispatch tests show
small absolute overhead in the new handlers. The earlier short, separate-batch
spot check did not reveal this; its broad no-slowdown interpretation should not
replace these more controlled results.

| Workload | Old median | New median | New time relative to old |
| --- | ---: | ---: | ---: |
| Navigate and render a 120x40 panel, 2,000-file directory | 1.774472 ms | 1.743280 ms | 1.76% lower |
| Root text-overlay rendering | 0.314945 ms | 0.314205 ms | 0.23% lower |
| List-down dispatch into an empty process panel | 20.215 ns | 21.175 ns | 4.75% higher |
| Help-menu wheel dispatch | 14.675 ns | 16.480 ns | 12.30% higher |

Nanosecond-level dispatch differences should not be presented as equivalent
percentage changes in whole-app scrolling. The keyboard difference is about one
nanosecond and the help-wheel difference about two. Rendered-frame median
differences are small and do not justify a promise of noticeably faster use.
This is a maintenance cleanup, not a demonstrated user-visible speed improvement.

Keyboard and help-wheel allocations remain zero. Overlay allocations remain 709
per operation. The file-list frame reports 10,562 allocations before and
10,561-10,562 after; effectively unchanged in this experiment.

## Method and correctness

- Go 1.26.5, Windows/amd64, Intel Core i5-1135G7; same build options and test source.
- The old test executable compiles the saved pre-cleanup `model.go`,
  `key_function.go`, `modal_mouse.go`, and `handle_panel_movement.go` through Go's
  overlay mechanism. Everything else is identical to the current checkout.
  These are the handlers exercised by the benchmarks; this is not a reconstruction
  of every old source file or a complete old-release executable comparison.
- One retained, exclusively created corpus is shared by both builds: 2,000 empty
  files with fixed indexed names, mixed file extensions and ASCII/Unicode names.
  Creation and directory loading are outside the timed frame loop. The benchmark
  measures one list-down action followed by rendering the visible panel, not
  disk traversal or scrolling through every file.
- One complete warm-up round is discarded. Twelve measured rounds alternate
  new-first and old-first order. Each workload has a 500 ms target per invocation.
  No competing build/test/performance workload was run during the timing phase.
- Existing overlapping-key/odd-dimension overlay checks pass in both builds.
  Three navigation-frame SHA256 fingerprints match exactly before timing.
- Medians, unit conversions and percentage arithmetic were independently checked
  with the verified technical solver. Its reasoning-ledger audit is structural,
  not statistical proof or proof that the application behaves identically in
  every possible situation.

This does not measure startup, native Everything search, disk I/O, terminal paint
latency, or every command. No statistical equivalence claim is made, and CPU
state and scheduling may still influence results.

## Artifacts and reproduction

Raw metadata, all timings and correctness logs:
`X:\Xelif-work\benchmarks\cleanup-comparison-fb3ec1aa763847df8d0eca6b220b4501`.
See `metadata.json`, `samples.json`, `old-correctness.txt`,
`new-correctness.txt`, and each `round-*-{old,new}.txt`.

Overlay mapping: `X:\Xelif-work\benchmarks\cleanup-speed-overlay.json`.
Runner: `X:\Xelif-work\benchmarks\Compare-Cleanup-Speed.ps1`.
Compile both root test executables with the same toolchain, adding `-overlay` only
to the old build. The runner creates a new corpus, checks parity, then benchmarks
both versions serially. The checked-in fixture is
`src/internal/speed_comparison_test.go`; ordinary tests skip corpus generation.

The installed application was not replaced or modified during this comparison.
Its SHA256 remains
`DA80A3B5824D89D2B601CCFE3DA1163B0DE8E070F9F96305AFCECAA33E145218`.
