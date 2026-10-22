# Xelif completion audit

Status: active development, not a completed release. This audit retains the recorded feature scope in README.md and the feature guide; it does not redefine completion as the latest UI fix. Historical passes in verification.md are checkpoints, not substitutes for a final-source run.

## Current finishing pass (2026-10-22)

This pass preserves the approved compact layout, monochrome default with purposeful
file-icon colors, single-row wheel movement, and mouse/keyboard access. The
interaction reference is OpenCode **v2**, specifically contextual commands and
predictable dialog focus, not a chat-style layout or ornamental animation. The
[v2 TUI guide](https://dev.opencode.ai/v2/docs/cli/tui/) and
[v2 keybinding guide](https://dev.opencode.ai/v2/docs/cli/keybinds/) describe those
command and dialog mechanics. Xelif retains its own configured bindings.

Concrete finishing changes:

- Commands opens workspace, saved-search, batch-rename, tags, collections and
  metadata prompts with the appropriate command prefix and syntax help. A picker
  confirmation cannot also submit the newly opened prompt. Successful previews
  receive focus even when ordinary prompt results are configured to stay open.
- Tag and collection results use the existing searchable, scrollable, clickable
  result picker. Choosing a path reveals that exact item in its local panel;
  missing or filtered-out files produce an inline error without losing the list.
  Opening a collection no longer silently jumps to its first member.
- Corrupt, unreadable or unsupported catalogs fail closed. Existing bytes are
  preserved; catalog writes are unavailable instead of silently starting empty.
- Copy-missing apply is bound to the reviewed action set and source identities,
  metadata and SHA-256 content. Changed inputs require another preview. Only
  reviewed actions execute; destination publication remains create-if-absent.
  Preview hashing is a deliberate safety cost, not a claimed speed improvement.
- ZIP work captures an immutable request and uses its own handle. Closing or
  replacing the picker cancels and invalidates queued work without waiting on a
  worker's read/extract lock. Stale/duplicate results cannot replace the current
  view. Current archive/copy/compare failures are visible above progress views;
  partial copies report retained creations.

The regression tests exercise real root dispatch and rendered overlays, not just
standalone package APIs. Final combined gates and measurement details belong in
[verification.md](verification.md); do not treat this list as native-terminal,
native macOS/Linux, live-remote or whole-application performance verification.

## Core requested experience

| Requirement | Current evidence | Remaining gate |
| --- | --- | --- |
| Approved compact UI, recognizable color-coded icons, aligned columns | Live Windows Terminal screenshots and focused filepanel tests recorded in verification.md | Final combined interaction pass after remaining changes |
| Mouse and keyboard access | Mouse/context/tab/header tests; live new-tab, Commands, selection, and sidebar checks | Requirement-by-requirement reachability audit of every added action |
| Independent single-row sidebar scrolling, last disk reachable | Live reduced-window check with information panels shown and hidden; refresh/partial-heading tests | Preserve behavior through final integration |
| Favourites using existing sidebar pinning | Live mouse pin/unpin, keyboard context menu and quit/reopen persistence; offline/corrupt-storage regressions | Native macOS/Linux interaction remains unverified |
| Exact build launched as Xelif from any directory | PowerShell/CMD/Git Bash launch checks, minimal-PATH check, live path-with-spaces launch | Machine-local setup only; WSL is not configured |
| Fast global filename search with direct Everything integration | Native IPC tests and local benchmark record | Refresh final live-provider check; no claim of outperforming Everything |

## Recorded expanded feature scope

The following are implemented in the working tree but are not accepted as globally finished solely because their packages exist. Their package suites and root integration tests must pass at the final source state, with live boundary checks where relevant.

| Feature | Evidence locations | Final acceptance still needed |
| --- | --- | --- |
| Session/workspace persistence, tabs/history, watched-folder refresh | session, folderwatch, filepanel and root tests | Combined save/reopen/navigation workflow |
| Advanced/saved search, selection, batch rename and undo | globalsearch, savedsearch, batchrename and root tests | Live preview/apply/cancel and failure-path audit |
| ZIP browsing, safe extraction, copy/move/paste | archivebrowser, dircompare and root operation tests; test-only command-completion synchronization and final serialized suite passed | Disposable live boundary checks; production archive progress/finalization ordering remains unchanged |
| Git status, diffs, stage/unstage, confirmed discard | gitstatus and root Git tests | Disposable-repository live mutation/confirmation checks |
| Duplicates and directory comparison/copy-missing sync | duplicates and dircompare suites | Final cancellation, no-clobber and UI integration checks |
| Tags/collections and metadata preview/apply/restore | catalog, metadataedit and root tests | Final interaction and journal-restoration checks |
| Plain mode, language foundation, inbound dropped paths | common/i18n and root tests | Display/terminal boundary checks; translations are not claimed complete |
| Declarative command plugins and read-only remote browsing/download | plugincommands, remote and corresponding UI/root tests | Final bounded-execution and remote-download boundary checks |

## Open quality gates

The earlier local Windows cleanup passed with zero active findings under
golangci-lint 2.11.4, using unchanged analyzer settings. Its report is
`X:\Xelif-work\lint-cleanup-final.json`; see `docs/lint-cleanup.md` for retained
intentional patterns and behavior/performance checks. Hosted 2.12.2 checks later
found additional duplicate literals and Unix-only findings. The portability
follow-up and its verification boundaries are recorded in
[verification.md](verification.md); the earlier report is not a final hosted
lint result.

- The serialized full short suite, raw vet and full layout sweep passed after lint cleanup. Earlier asynchronous-test failures and their synchronization changes are recorded in verification.md; a universal flake-free or race-free claim is not made. The current finishing pass's website production build passed (42 pages, with 38 pages indexed in two languages). Final combined live acceptance remains open, and source checks must be refreshed after further changes.
- Windows, Linux/amd64 and macOS/arm64 builds were refreshed after lint cleanup. Cross-compilation is not native runtime testing; rebuild again after further source changes.
- Reconcile the chronological verification record with final results and inspect current GitHub state. No commit, repository creation, push, release, or remote CI run is authorized merely by this audit.

Completion requires evidence for all accepted requirements, not only a clean launch or a green subset of tests.

### Favourites acceptance refresh (2026-10-23)

An isolated candidate passed real Windows Terminal checks: mouse pin/unpin,
sidebar keyboard context menu, persistence across quit/reopen, one-row file and
sidebar wheel movement, viewport stability across refresh, and reaching the final
disk with information panels both hidden and shown. Current-folder fallback
search returned and opened a directory result. The approved layout was reused;
existing user settings and favourites were not changed by these tests.

The Everything comparison remains open. Its service was running in session 0;
starting the existing desktop executable did not expose the native query window.
The local-only 10,000-file benchmark passed exact-set, Unicode and cancellation
checks, but `ComparisonValid` was false. No provider ranking or Everything-level
speed claim is justified. No npm/Bun packaging work was performed.

Safety correction after the serialized full-suite checkpoint: ordinary file copying now creates destination files exclusively rather than truncating a destination that appeared after duplicate-name selection. A regression test first reproduced the overwrite, then passed after the correction; new-destination copying, paste/compression/undo regressions, trash/dircompare/duplicates suites, raw full vet, and the Windows build passed. Three unreachable legacy copy/move helpers were removed. The rebuilt binary was installed and its hash matched the verified build. The full suite must be refreshed again after this and subsequent changes.

GitHub refreshed on 2026-10-22: authenticated as `ASVLCII`; origin is
`https://github.com/ASVLCII/Xelif.git`, public, default branch `main`. Upstream is
retained separately. The published tip checked was `76c3645` (README/demo), before
this finishing pass. Its Go and Python workflows failed; module-tidy passed.
Portability/harness fixes are part of this working-tree pass and still require
hosted native validation after publishing. Older repository-state observations
in the chronological verification log are historical, not the current state.
