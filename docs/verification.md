# Xelif verification checkpoint

## Finishing-pass verification (2026-10-22)

This development snapshot preserves the approved compact layout and wheel
behavior. It is not a claim that every recorded feature has completed native
end-to-end acceptance. The scope and remaining live boundaries are tracked in
[completion-audit.md](completion-audit.md).

Final-source local checks passed:

- Serialized full short suite: `go test -p=1 -short -count=1 -timeout=8m ./...`
  (root package 80.885 seconds), plus a separate full `TestLayout` sweep
  (25.674 seconds). Logs: `X:\Xelif-work\standout-serial-final-suite.log` and
  `standout-final-layout.log`.
- Raw `go vet ./...` and configured repository-wide golangci-lint 2.11.4:
  both exited zero; lint reported **0 issues**. No global analyzer weakening
  or new file exclusions were added. Logs: `standout-final-vet.log` and
  `standout-final-source-lint.log` under `X:\Xelif-work`.
- Five mocked Python harness tests passed from the same working directory and
  command used by CI. They do not replace the native Linux tmux integration run.
- Website production build passed: 42 static pages; Pagefind indexed 38 pages
  in two languages. Log: `X:\Xelif-work\standout-website-build.log`.
- Output parity and paired performance measurements are recorded in
  [standout-render.md](benchmarks/standout-render.md): 24,334 sanitizer inputs
  and the 96-frame display-mode matrix match the baseline. Checked root
  navigation/render workloads allocate 3.6–3.7% fewer temporary bytes, with
  approximately unchanged timing, not an established whole-app speedup.

An earlier combined run overlapped project builds and failed compression
completion waits and a shell-substitution assertion. Both groups then passed
three isolated repeats with their original deadlines, followed by the passing
serialized full suite above. Resource contention is plausible, not an established
cause. These results do not establish universal flake-free or race-free tests;
pre-existing tests still include direct model access around an event loop.

The opt-in Windows Everything IPC smoke reported `everything is not running`.
It did not silently use a CLI or local-search fallback. Successful live provider
queries, renewed Windows Terminal mouse/paint checks and native macOS execution
are therefore not claimed for this pass. Unit/integration fixtures and
cross-compilation are distinct from those live boundaries.

Installation and final artifact verification (2026-10-23): the Windows/amd64,
Linux/amd64 and macOS/arm64 binaries were rebuilt from the final source. All
302 non-test Go/module/embedded-asset inputs still match the build-start manifest,
whose aggregate SHA-256 is
`F5D5C6AC658E5AAE664E99FD11B53D9BC93EC84A783F7AFCE5BB1953CAD2F4C4`.
Artifacts and input manifest are retained in
`X:\Xelif-work\standout-final-builds\run-20261022-231407-5f51c2c8`.
The Windows candidate rendered the welcome and file browser in an isolated PTY,
then quit normally with exit code zero. This was not a native Windows Terminal
mouse or visual acceptance run.

The installed `X:\Xelif\bin\spf.exe` matches that verified Windows artifact,
SHA-256 `627BA794157A1A96A59F2C7BFC3327F8C3FCE446CF454FD7297317EC44B241A1`.
The previous executable was hash-verified and backed up as
`X:\Xelif-work\spf-before-standout-20261023.exe`. The existing launcher dependency
check passed; launcher scripts and the normal runtime profile were preserved.

GitHub publication is explicitly authorized to `ASVLCII/Xelif` on `main`.
Authentication was rechecked as `ASVLCII`; the fetched remote matched the starting
commit before publication. This entry records local verification; hosted CI
must be inspected separately for the published commit.

The older records below are chronological checkpoints and may describe
superseded repository, installation, lint or authorization state.

## Earlier local checkpoint (2026-10-21)

Local development checkpoint: 2026-10-21. This is not a release or a declaration that all requested work is finished. Later source edits require rerunning the affected checks.

## Verified locally

| Gate | Command or evidence | Result |
| --- | --- | --- |
| Full short Go suite | `go test -short -count=1 -timeout=5m ./...` | Passed, including all P0/P1/P2 package suites. The layout sweep and opt-in live tests are separate. |
| Full layout sweep | `go test -count=1 -timeout=8m ./src/internal -run=^TestLayout$` | Passed again after tab overflow and modal fixes; covers the repository's 800-plus layout combinations. |
| Static checks | `go vet ./...` | Passed after the search, tab-overflow, modal, and test-isolation changes. |
| Windows build | `go build -o bin/spf.exe .` | Passed after the search, tab-overflow, and modal changes. |
| Documentation | `pnpm build` in `website` | Passed: 42 pages generated; Pagefind indexed 38 pages in two languages. Astro emitted deprecation warnings. |
| Scoped strict lint | `golangci-lint run ./src/internal/ui/metadata/... ./src/internal/common/... ./src/internal/remote/...` | Passed with zero issues after applying repository formatters and fixing an error-variable shadow. This is not a full-repository lint result. |
| Search/tab strict lint | `golangci-lint run ./src/internal/ui/globalsearch/... ./src/internal/ui/filepanel/...` | Passed with zero issues. Includes narrow native-ABI safety explanations and opt-in benchmark-only complexity exclusions; no global analyzer disabling. |
| Remote safety | `go test -count=3 ./src/internal/remote` | Passed, including no-clobber publishing, error classification, cancellation, and bounded output. |
| Welcome and chooser regressions | `go test ./src/internal -run='TestFirstUseFitsStandardTerminal\|TestChooser'` | Passed. Welcome content fits an 80 by 24 terminal; chooser returns all selected paths and does not quit on write failure. |
| Live terminal smoke | Launched `bin/spf.exe` with isolated XDG config/data/cache/state directories under `X:\Xelif-work\smoke` | Opened the file manager and search; current-folder fallback was visibly identified; normal quit exited successfully. |
| First-run theme | Inspected isolated generated `config.toml` and `theme/monochrome.toml` | Monochrome selected and installed. Unit tests also verify neutral RGB values and preservation of customized themes. |
| Live native Everything IPC | `XELIF_TEST_EVERYTHING=1 go test -count=5 -timeout=30s ./src/internal/ui/globalsearch -run=^TestEverythingNativeLive$ -v` | Passed five consecutive queries, 0.29-0.31 seconds each, returning indexed `explorer.exe` paths with the requested result cap. This is a live integration smoke check, not a comparative benchmark. |
| Search reliability | Full `globalsearch` suite with `XELIF_TEST_EVERYTHING=1`, three repeats; later full short suite and focused search suite | Passed after per-provider budgets, cancellation classification, CSV result caps, unique reply IDs, and bounded CLI output capture. |
| Final live TUI search | Rebuilt Windows binary, isolated XDG directories | Ctrl+G returned indexed paths including `C:\Windows\explorer.exe`; an injected terminal mouse click on Search opened the same dialog. Normal quit exited successfully. |
| Tab and modal regressions | Integrated mouse/tab tests plus Unicode overflow tests at widths 18, 24, 40, and 80 | Passed. Active tabs and controls remain visible; wheel reaches hidden tabs. Sort cancellation uses rendered bounds, and hidden sort dialogs do not intercept higher-priority overlays. |
| Cross-platform compilation | `CGO_ENABLED=0 go build` for Linux amd64 and macOS arm64 | Both refreshed successfully after tab/navigation and CLI-output changes. They predate the last result-limit notice adjustment; no native Linux/macOS execution was performed. |

The remote test fixture originally imposed a one-second operation deadline unrelated to its filesystem-safety assertions. One loaded full run hit that deadline. The fixture now uses the production default budget; explicit timeout/cancellation classification tests remain intact. Production deadlines were not increased.

The root test suite also previously reused one fixed temporary directory across invocations. Concurrent checks could delete or lock each other's fixtures. TestMain now creates a unique per-run directory; the full short suite passed afterward.

## Compact-symbol revision (2026-10-21)

The user rejected the three-row inline thumbnails after seeing the real application. That implementation, its Sixel output adapter, background loader/cache, toggle, capability probing and settings have been removed. All file entries now use the compact one-row geometry. Shared mouse hit-testing, tiny-panel/blank-row safety, the range-selection context action, and Windows Terminal-friendly alternate keys are retained.

New profiles use font-independent file-type markers; existing custom Nerd Font preferences are preserved. Actual images remain available in the separate large preview. The historical thumbnail test results no longer describe the current implementation.

Compact-list verification: common/filepanel/preview tests, root mouse/context/plain tests, the layout sweep, scoped common/filepanel lint, website build, and full vet passed. The full short suite passed all packages except four stale prompt-title snapshots; those snapshots were updated for font-independent title widths and the prompt package rerun passed. Live Windows Terminal checks passed marker clicking, keyboard next-row navigation, Ctrl-selection, right-click context menus, and exact single-row spacing.

Wheel revision: removed the five-action multiplier from main-panel wheel handling. Search/palette, archive, and remote lists now move one item per wheel event; text-preview scrolling and keyboard navigation are unchanged. Regression mouse/wheel/context tests, full vet, and the rebuilt Windows executable passed. Native Windows Terminal wheel input was checked in both directions: one notch moved from demo.png to notes.txt and one notch back returned to demo.png. The rebuilt app was left open using the isolated test profile; normal user configuration was not changed.

Unicode-symbol follow-up: replaced bracketed letter markers with monochrome Unicode (`▸`, `▧`, `≡`, `▥`, `□`, `↗`). Existing Nerd Font preferences and text-only plain mode remain unchanged. Common and filepanel tests passed, including a one-terminal-column assertion for every symbol. The Windows build passed, and live terminal accessibility output confirmed folder/image/document symbols and single-row spacing. The screenshot was occluded, so visual glyph rendering was not independently confirmed. No file-row geometry or wheel behavior changed.

Final color-icon checks: scoped common/filepanel lint reports 0 issues; common/filepanel tests passed again after lint cleanup, the Windows binary was rebuilt, and the website build passed (existing Astro deprecation notices remain). Full-repository tests/lint were not rerun for this bounded color-only change.

## Incomplete gates

Native/copy safety pass: reproduced an ordinary-copy destination-clobber issue with a failing regression test, then changed copyFile to use exclusive creation and report final close failures. Existing destinations and the source are preserved by the regression check, while a separate check verifies new destinations. Removed three unreachable legacy copy/move helpers after searching all source references. Focused copy/paste/compression/undo tests and trash/dircompare/duplicates suites passed; raw `go vet ./...` and the Windows build passed. The verified executable was installed at `X:\Xelif\bin\spf.exe` and matched the build hash.

Windows COM cleanup: HRESULT failure checks now inspect the 32-bit severity bit without narrowing casts; formatting explicitly masks to HRESULT width. CoUninitialize's void return is explicitly ignored. Callback receivers, interface IDs, and out-parameters use pointer-typed callback arguments, eliminating uintptr-to-pointer suppressions and passing a test that invokes the actual native callback thunks for QueryInterface/Release. Callback receiver lifetime now extends through native COM-object release on every return path. This test does not perform an actual Recycle Bin deletion. References: [HRESULT structure](https://learn.microsoft.com/en-us/windows/win32/com/structure-of-com-error-codes) and [CoUninitialize](https://learn.microsoft.com/en-us/windows/win32/api/combaseapi/nf-combaseapi-couninitialize); callback argument support was checked in the installed Go runtime's syscall_windows.go. Full lint now reports 67 remaining findings, saved at `X:\Xelif-work\lint-safety-current.json`, rather than the previous 102. No global analyzer disabling was added. Full final-source tests and cross-platform builds remain required after the outstanding work.

Completion audit refresh (2026-10-21): `go test -p=1 -short -count=1 -timeout=8m ./...` completed successfully with exit code 0. Root compression/paste tests also passed three focused repeats without increasing their one-second waits. This demonstrates a passing serialized run, not an established explanation for the earlier parallel failures. Configured repository formatters ran, the directory-copy executor's embedded field was reordered, and a cancellation assertion now uses errors.Is. Directory-compare/duplicate suites passed. Repository-wide lint dropped from 153 to 102 findings; structured remaining issues are in `X:\Xelif-work\lint-after-format.json`. Completion scope and outstanding live/safety gates are tracked in `docs/completion-audit.md`. GitHub read-only check authenticated as ASVLCII; origin remains upstream and ASVLCII/Xelif could not be resolved. No publishing occurred. The overall goal remains active.

Column-spacing revision (2026-10-21): Size header/data are right-aligned in an 11-cell column; Modified header/data are left-aligned in a 16-cell column, with three-cell column separators. Unknown folder sizes show an em dash (ASCII hyphen in plain mode), not blank cells or a false zero. Actual zero-byte files still display `0 B`. The Modified header retains date sorting. Focused filepanel tests, scoped lint (0 issues), Windows build, and live terminal header/placeholder checks passed. Screenshot: `X:\Xelif-work\thumbnail-proof\column-spacing.png`.

Broader audit refresh: Linux amd64 and macOS arm64 cross-compilation completed successfully before the column-spacing edit; no native execution is implied. Full repository lint completed with 153 findings (including 50 import-formatting findings and root-model complexity, Windows COM, and other analyzer findings); it remains an open gate. The full short test run passed every package except the root package, where compression/extraction and paste assertions timed out while waiting for results. The cause has not yet been established; these failures are not classified as harmless or resolved. This broader test run overlapped the spacing edit, so final affected-package checks are recorded separately above.

Independent-sidebar-scroll correction (2026-10-21): mouse wheel now moves the sidebar viewport by exactly one terminal row, independently of selection. A row offset within multi-row headings keeps scrolling and click hit-testing aligned even when only part of a section heading is visible. Viewport limits clamp without wrapping, resizing clamps excess scroll, and keyboard navigation returns to selection-following behavior. Routine sidebar refreshes now preserve the viewport when entries are unchanged, instead of recentering on the selected directory after every message.

Verification: sidebar suite and root mouse/wheel/context regression tests passed; full vet and Windows build passed. Live Windows Terminal test in a deliberately reduced 1000x540 window reached the last disk (`Z:`) with the information panels both hidden and shown, confirmed exactly one visible row per native wheel notch, checked stability across background refreshes, and checked the bottom boundary. The test restored the original footer visibility setting. The rebuilt binary is installed at `X:\Xelif\bin\spf.exe`, the same executable selected by the `Xelif` command. Full-repository test/lint were not rerun for this bounded fix.

Approved-layout revision (2026-10-21): implemented the generated preview's single-tab path header, right-aligned create-tab control, unbracketed Search/Commands actions, `Filter files` placeholder, stable always-visible sidebar Search, and colored sidebar/Recent glyphs using the existing palette. Removed shortcut hints from both search inputs. Bottom panels now default hidden, while existing saved preferences and the keyboard toggle remain available; border statistics default hidden too. Multi-tab controls still appear when multiple tabs exist, and compact path components retain ancestor click targets. Mouse wheel now targets the hovered sidebar/file panel and stops at list boundaries; sidebar wheel skips section headings as one directory step without keyboard-style wraparound.

Verification: common/filepanel/sidebar suites, root mouse/wheel/context regressions, full vet, Windows build, scoped lint (0 issues), and diff whitespace checks passed. All packages reported passing in the full short run, but its command exited nonzero because Windows denied deletion of one temporary plugincommands test executable; this is not recorded as a clean full-command success. The affected UI packages passed again after final presentation fixes. Native Windows Terminal checks passed six individual downward sidebar notches through adjacent entries, one-notch reverse, repeated top-boundary scrolling without wrap, right-aligned new-tab clicking, Alt+W close-tab, and clickable Commands opening/dismissing the palette. Final visual evidence: `X:\Xelif-work\thumbnail-proof\clean-layout.png`. No publishing or normal-profile configuration rewrite was performed.

Color-icon revision (2026-10-21): implemented the researched categorical palette documented in `docs/icon-colors.md`, retaining original Nerd Font glyphs and neutral filenames/interface. Added `color_file_icons` (enabled by default, disabled for fully monochrome icons), explicit background contrast selection, selected-background handling, and plain-mode/NO_COLOR/transparent-background fallbacks. Common/filepanel suites, scoped vet, mouse/wheel/context regressions, and Windows build passed. Tests cover category aliases, uppercase extensions, links/folders, normal/selected/light backgrounds at the 4.5:1 target, configuration opt-out, and background uncertainty. The rebuilt app was opened through the Xelif Windows Terminal profile; `X:\Xelif-work\thumbnail-proof\colored-icons.png` visually confirms blue folder icons, green Markdown, cyan technical-file icons, neutral filenames, and unchanged single-row entries. Earlier monochrome-icon checkpoints are superseded by this explicit user request. No commit or publishing was performed.

Nerd Font correction: the user explicitly requested the recognizable original icon set rather than abstract Unicode substitutes. Restored `nerdfont = true` for new profiles and enabled it in the isolated runtime profile. Existing normal user config already had Nerd Font support enabled and was not rewritten. Installed CaskaydiaMono NFM Regular from the official Nerd Fonts v3.5.1 CascadiaMono release for the current Windows account, and added a dedicated Windows Terminal Xelif profile with that font; other terminal profiles and defaults were preserved. Common/filepanel tests and the Windows build passed. A new regression test checks original folder/image/text/file glyphs and monochrome coloring. Reopened the rebuilt binary and visually confirmed recognizable monochrome icons in `X:\Xelif-work\thumbnail-proof\nerd-icons.png`. Single-row geometry and one-item wheel handling were not changed. Launch with `wt -p Xelif`.

- Complete the requirement-by-requirement final audit and full-repository lint review. Search/navigation checks, full short suite, full vet, and the layout sweep have passed; affected search tests were repeated after the last result-limit notice adjustment. Nexus workers were stopped after editing-tool failures and excessive coordination overhead; their incomplete results are not accepted as implementations.
- Resolve or explicitly classify full-repository lint findings, and refresh cross-platform builds. The full lint attempt found formatting and analyzer issues outside the clean scoped modules, plus a transient typechecking error caused by sources changing during its run; do not treat that attempt as a final lint result.
- Repeat actual terminal interaction and visual checks after the final changes; generated design previews are not runtime evidence.
- Preserve graceful native-provider startup behavior. Initial live attempts reported `Everything is not running`; a later Win32 probe confirmed its search window was reachable on WinSta0/Default and subsequent live suites passed, including three repeats after the final result-limit notice adjustment. The exact startup-delay cause has not been established. Current-folder fallback remains explicit when the application is unavailable.
- Audit the entire requested feature scope against runtime behavior and test coverage, including mouse and keyboard reachability.

The existing search benchmark record in [benchmarks/search.md](benchmarks/search.md) is local-only. It does not prove Xelif is faster than Everything.

## Action-validation verification (2026-10-22)

Corrected the metadata regression to assert the notification dialog actually used by `showUserError`. The test preserves the existing journal and verifies no result viewer opens for a rejected preview action. Navigation/selection/sorting hotkey lookup is separated from operation hotkeys without changing configured bindings. Help-menu wheel dispatch now uses an explicit button switch; its existing scroll amount and file/sidebar one-row behavior are unchanged.

At this source checkpoint, focused metadata/batch/archive/palette/rename tests, raw `go vet ./...`, the serialized full short suite (`go test -p=1 -short -count=1 -timeout=8m ./...`), the separate full layout sweep, Windows build, and diff whitespace checks all passed. The installed `bin/spf.exe` matches `bin/spf-actions-new.exe`, SHA256 `3BE2F8B483F45EE3800837CD4983D884202F0E561EB08D1C0DB0F6DB02ED985D`. Launcher dependency checking passed. This checkpoint does not claim a new live terminal interaction pass or refreshed cross-platform builds.

Repository-wide lint still fails: 52 findings in `X:\Xelif-work\lint-actions-verified.json` (3 funlen, 12 gocognit, 1 goconst, 8 intrange, 1 maintidx, 11 mnd, 1 nilnil, 14 reassign, 1 unparam). No global analyzer disabling was used. The broad completion audit remains open; nothing was committed or published.

## Optimization checkpoint (2026-10-22)

Measured, behavior-preserving changes: size sorting caches immediate child counts
only for the duration of one sort; fixed icon-palette luminances are precomputed
without caching theme backgrounds; disabled debug logging no longer collects
runtime memory statistics. Details, raw-output locations, and measurement limits
are in [benchmarks/optimization.md](benchmarks/optimization.md). No UI controls,
wheel steps, search-provider order, refresh intervals, or dependencies changed.

The first full run hit a compression/extraction timeout and locked cleanup. Test
completion observers now wait for actual command result messages rather than
process-bar success/file existence; focused tests passed five repeats with the
original timeout. Production archive code is unchanged. The final isolated full
short suite, full vet, layout sweep, Windows build, Linux/amd64 cross-build and
macOS/arm64 cross-build passed. Six rendered-output fingerprints match the baseline
and color parity tests pass. Lint remains nonzero with 52 existing findings in
`X:\Xelif-work\lint-optimization-final.json`; no new findings were introduced.

The installed `bin/spf.exe` matches the verified performance build, SHA256
`BA6CE2A63B074A99EB8E46CDACD397162ACECE81DD841FCC8EF381383E9FEBAA`.
Launcher dependency checking passed. A candidate window rendered the expected
interface, but renewed live wheel testing was blocked by Windows denying
`SetCursorPos`. Everything's query window remained unavailable after an attempted
startup of the installed application, so no live search latency or successful
native-provider check is claimed. Repository creation, commits and publishing
were not performed. The overall completion audit remains open.

## Careful lint-cleanup checkpoint (2026-10-22)

Repository-wide lint now exits zero with 0 active findings, using the existing
configuration (`X:\Xelif-work\lint-cleanup-final.json`). No global analyzer
disabling or new file exclusions were added. Meaningful helper extraction, named
constants, stable-bound integer ranges, and test-helper simplification address the
structural findings. Intentional test-state overrides and successful optional
undo results are narrowly documented and tested rather than behavior-changed.
See [lint-cleanup.md](lint-cleanup.md) for scope and performance evidence.

The final serialized full short suite, raw vet, full layout sweep, Windows build,
Linux/amd64 cross-build and macOS/arm64 cross-build passed. Source hotkey-check
order is unchanged; new overlap and odd-dimension placement tests pass. No
slowdown was detected in the checked navigation/help-wheel/overlay benchmarks,
and allocation counts were unchanged. These spot checks do not prove every path
has identical timing or establish a whole-app speedup.

The installed `bin/spf.exe` matches the verified cleanup build, SHA256
`DA80A3B5824D89D2B601CCFE3DA1163B0DE8E070F9F96305AFCECAA33E145218`.
Launcher checking and diff whitespace checking passed. No new live desktop or
Everything-provider check is claimed; the broader acceptance audit remains open.
Nothing was committed or published.

## Historical GitHub state (before publication)

Read-only inspection authenticated as `ASVLCII`. Local branch: `xelif/p0-lovable-explorer`; origin still points to `yorukot/superfile`. No owned repository matching Xelif or superfile was found. Upstream main is ten commits beyond the checkout's original base; no wholesale merge was performed. No Xelif repository, commit, push, release, or remote CI run was created by this checkpoint.

Upstream CI and release status are separate from Xelif verification. Publishing requires separate authorization and identity/remote checks.
