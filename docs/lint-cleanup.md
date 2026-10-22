# Careful lint cleanup (2026-10-22)

The baseline was 52 active repository-wide findings. The existing lint settings
were retained; no global analyzer or file exclusion was added. The final lint
invocation reports zero issues (`X:\Xelif-work\lint-cleanup-final.json`).

## Scope and behavior safeguards

- Split event-result handling, folder-watch dispatch, quit finalization, overlay
  placement, mouse targets, palette actions, Git status work, and copy branches
  into focused helpers. Retain command construction, operation/error order,
  asynchronous captures, and main command-batch order.
- Retain the flat, ordered hotkey switch rather than regrouping potentially
  overlapping custom bindings. All 80 hotkey checks in the file retain their
  original order. Existing flat-dispatch length/branch exceptions are explained;
  the maintainability finding is resolved without suppressing that analyzer.
- Reuse the existing Windows OS constant, give fixed UI/protocol values meaningful
  names, and use integer-range loops only where bounds remain stable.
- Keep intentional global path overrides local to tests with cleanup restoration
  and narrowly justified annotations. Keep `executeUndo`'s successful `nil, nil`
  contract rather than inventing an error; new tests distinguish completed, empty,
  and missing journals. These are documented intentional patterns, not bug fixes.
- Preserve copy-cancellation assertions, including file identity and retained
  contents. New overlap-binding and odd-dimension overlay tests guard refactoring
  boundaries. The optional-command helper return changes retain handled-nil
  behavior, so a completed action cannot fall through to another command.

No intended UI, shortcut, scroll step, result cap, refresh interval, feature,
dependency, or file-operation behavior change is part of this cleanup.

## Performance spot checks

Go 1.26.5, Windows/amd64, Intel Core i5-1135G7. Same fixtures and command before and
after: `go test -run=^$ -bench='BenchmarkMainNavigationDispatch|BenchmarkHelpWheelDispatch|BenchmarkRootOverlayRender' -benchmem -benchtime=300ms -count=5 ./src/internal`.

| Fixture | Before median | After median | Allocations before/after |
| --- | ---: | ---: | ---: |
| Main list-navigation dispatch | 38.36 ns | 21.43 ns | 0 / 0 |
| Help wheel dispatch | 18.28 ns | 16.88 ns | 0 / 0 |
| Root text-overlay rendering | 405.524 us | 319.109 us | 709 / 709 |

These short local batches show no slowdown in these fixtures. They do not prove
every action is speed-neutral or attribute all timing differences to cleanup;
CPU state and scheduling vary. They are not whole-application speedup claims.
Raw runs: `X:\Xelif-work\benchmarks\lint-cleanup-{before,after}.txt`.

These initial separate-batch spot checks are superseded for performance
interpretation by the [paired old/new comparison](benchmarks/cleanup-speed-comparison.md).
That test finds similar full-frame medians but roughly one to two nanoseconds of
overhead in the isolated keyboard/help-wheel dispatch tests. Do not read the
initial spot check as a guarantee of zero slowdown in every path.

## Verification record

Focused affected tests and root mouse/modal/tab/history/overlap tests passed.
The final serialized full short suite, raw vet, full layout sweep, Windows build,
Linux/amd64 cross-build and macOS/arm64 cross-build passed. Cross-builds are not
native runtime execution. Diff whitespace checking and launcher dependency checks
passed. The installed Windows binary matches the verified build, SHA256
`DA80A3B5824D89D2B601CCFE3DA1163B0DE8E070F9F96305AFCECAA33E145218`.

Raw full-suite output: `X:\Xelif-work\lint-cleanup-full-tests.txt`. The live
desktop/Everything checks were not repeated in this pass; earlier environment
limitations are not presented as new live verification. Existing unrelated work
is preserved. Nothing was committed or published, and the broader project
completion audit remains open.
