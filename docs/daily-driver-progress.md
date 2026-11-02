# Daily Driver development checkpoint

This is **not a completed milestone or release approval**. The approved Daily
Driver plan remains the acceptance contract. Source stays in `X:\Xelif`; fixtures,
preserved binaries and reports stay in `X:\Xelif-work`.

## Baseline and preservation

- Clean source fast-forwarded from `7ebd19a4` to verified main
  `4e61438a89189cec22fc6993e70d7edde86cd6ff`. Recovery branch:
  `codex/pre-daily-driver-20261101`. No reset or force-push.
- Independent archives retained for this source and upstream Superfile v1.6.0
  (`865dba26c1d8d35b95bd7b7501ebdeac66ce2618`).
- Installed executable, launchers and runtime profile are not upgraded. Installed
  baseline reports v0.1.0; source main reports v0.1.2. No publication occurred.
- Artifact root: `X:\Xelif-work\daily-driver-20261101`.

## Implemented slice

- Full-window workspace strip, shared hit geometry, overflow picker, new/rename/
  close/reorder/reopen. Last close creates a fresh launch-directory workspace.
- Independent value snapshots for panel layout, navigation, sort, cursor, scroll,
  filter, selection identities and search. Clipboard/tasks remain global; explicit
  operations continue. Ephemeral replies are invalidated across switches.
- Version-2 app session, version-1 migration with exact-byte backup, verified
  previous backup, corrupt/unknown/duplicate-key protection. Named presets open a
  new workspace rather than replacing other open work.
- Combined filter/action row, optional preview, responsive sidebar drawer and
  focused-only split fallback that retains underlying panels. Legacy folder tabs
  remain available through a picker and commands, not a competing default strip.
- Xelif-owned profile roots, explicit whitelisted import with backups/recovery
  journal, explicit rich/portable appearance, optional legacy keymap, familiar
  new-profile shortcuts and side-effect-free `doctor --json`.
- Explicit indexed/folder scopes. Existing Everything IPC retained; Spotlight and
  plocate adapters use bounded cancellable NUL output and literal queries. Missing
  or unsupported indexes never silently become narrower filesystem searches.

## Verification evidence and limits

Approved generated previews are in `previews/` beneath the artifact root.
`candidate-render-v1/frames.json` exports the actual Go renderer respecting
inherited `NO_COLOR=1`; `candidate-render-color-v1` is a separate color-enabled
sample. These are **not native terminal screenshots or interaction evidence**.

`xelif-development.exe` is a separate development build, honestly reporting
source v0.1.2 with modified build metadata, not a published v0.2 candidate.

The complete short suite passed during integration. Focused Daily Driver tests
cover all four required reference sizes, split restoration, stale replies,
selection identity and lifecycle behavior. Profile/doctor tests check preservation,
appearance round trips and read-only dispatch. Vet passed during integration.
The nested extraction module suite and focused native Windows race suites passed.
The first non-short sweep caught a zero-panel capacity edge with a 24-column
legacy sidebar; capacity now retains at least the mandatory single panel.
Re-run final checks after later edits; do not treat intermediate green results as
final frozen-revision acceptance.

Profile tests caught and fixed a real appearance-writer token-range error. Go's
generated wrapper already propagates TestMain results; an unnecessary exit-code
change was removed after checking the toolchain source. Legacy workflow fixtures and
Daily Driver fixtures are explicit separate layouts; neither contract is dropped.

Three workers were used. Two stopped at usage limits, so their partial profile
and search changes were reviewed and tested by the parent. Nexus supplied a
bounded conceptual safety review at its supported max variant, not a source audit.

Useful verification commands (Go 1.26.5, isolated caches):

```text
go test -mod=readonly -short -count=1 -timeout=5m ./...
go test -mod=readonly -count=1 -run 'TestWorkspace|TestFirstUse' ./src/internal
go test -mod=readonly -count=1 ./src/cmd ./src/internal/profile
go vet -mod=readonly ./...
git diff --check
```

## Open gates — P0–P7 must not be declared complete

1. Final unchanged strict lint/formatters, full non-short suite/layout sweep,
   nested extraction modules, native race suites and installation matrix on one
   frozen revision. Cross-builds are not native runtime checks.
2. Controlled startup/navigation/CPU/memory/physical-input measurements and
   equivalent Everything/Superfile comparison. No current speed ranking or
   Everything-level reliability claim is justified by unit tests.
3. Full mutation-safety matrix, cross-volume/cancellation/shutdown stress and
   repeated workspace/search/preview resource measurements.
4. Real Windows 10/11 Terminal, macOS Terminal.app and Ubuntu GNOME/VTE interaction;
   appearance/import/recovery/custom-override states; missing-font and Unicode
   checks. No macOS/Linux desktop access has been established here.
5. Five user daily-use sessions, RC version availability, exact successful CI
   artifact provenance, clean npm/Bun installs and release authorization before
   publishing or replacing the installed command.

Next checkpoint: finish combined local checks, push the authorized development
checkpoint for native CI, then inspect its results. Publication of a stable
release, npm packages and installed-command replacement remains gated separately.

## Latest observed check results

- Repository-configured formatting applied to changed Go files; vet and Windows
  development build passed. Final complete short suite passed (root UI 72.400s).
- Full non-short legacy layout sweep passed (25.063s). Nested `third_party/xtractr`
  suite passed (0.337s). The earlier full non-short run's only failures were the
  subsequently repaired zero-capacity layout cases; do not call that earlier run
  itself green.
- Focused Windows race suite passed for root workspace lifecycle, profiles,
  sessions, search and worker shutdown. Follow-up workspace/filepanel race tests
  passed after adding the replacement-identity regression.
- Windows amd64 development build and Linux/macOS amd64/arm64 plus Windows arm64
  cross-builds passed during development. They are not frozen release artifacts
  and are not native desktop acceptance.
- Strict lint 2.12.2 was downloaded from its official release and checksum
  verified. Cold runs exceeded their timeout; warmed full analysis subsequently
  completed and reported 108 findings. The next complete analysis reported 83.
  Further reviewed fixes are in progress; unchanged strict lint is not yet clean.
- The recorded installed executable and runtime config/hotkeys/favourites hashes
  match the P0 inventory. No launcher replacement, profile import or publication
  was performed on the user's live installation.

## Resumed execution evidence — 2026-11-01

Artifacts below are under `X:\Xelif-work\daily-driver-20261101`.
These are development checkpoints, not acceptance of a frozen release.

- `native-acceptance-1/smoke-results.json`: actual Windows Terminal checks
  passed for explicit portable setup, Enter/Backspace navigation, keyboard and
  mouse workspace creation, bounded folder search, result reveal/resume and
  cancelled rename. Its executable is the preceding checkpoint, not later edits.
- `native-acceptance-1/interaction-results.json`: one injected Windows wheel
  notch advanced one file; narrow layout exposed the sidebar drawer and final
  disk, and widening restored the sidebar. This is not a physical wheel test.
- The configured Terminal font, `CaskaydiaMono NFM`, produced a missing-font
  warning. The warning was dismissed only in the dedicated test window. Fonts
  and Terminal settings were not changed; native rich-icon acceptance stays open.
- `baseline-comparison-2/results.json`: preserved Xelif `4e61438a` versus
  Superfile v1.6.0, matching 2,000-entry directory membership, order and focused
  identities. One warmup was excluded and 12 alternating samples retained.

| Isolated 120×28 panel benchmark | Old Xelif median | Superfile median |
|---|---:|---:|
| Navigation plus rendering | 0.985 ms/op | 2.023 ms/op |
| Forced directory reload | 1.600 ms/op | 1.599 ms/op |

These internal test-binary measurements are not terminal paint latency, input
p95, startup, native search, or the current Daily Driver candidate. No current
candidate or Everything-equivalent performance claim follows from them. The
first comparison attempt failed because the harness omitted the upstream search
shortcut initialization; no timings from that failed attempt were ranked.

Parent integration tests passed for resize safety, sort menus and workspace
selection identity. Filtered-out selections and their ordering/anchor now survive
workspace snapshots. A broader affected-package short suite passed (root UI
69.706s) before subsequent metadata/recovery integration edits; rerun afterward.

Nexus is supplying bounded source review and owned profile/search/workspace
patch proposals using free models. Every accepted proposal is inspected and
applied by the parent; worker assertions alone are not acceptance evidence.

## Follow-up integration and native evidence

This section supersedes earlier intermediate results only where explicitly
stated. All artifact paths are local beneath the artifact root above; they are
not published release artifacts.

### Correctness fixes

- Workspace context actions now resolve stable IDs without activating an
  inactive tab. Closing/reordering preserves the current workspace; closed
  snapshots are deeply copied and bounded to ten. Generated and manual titles
  respect durable byte limits without splitting terminal graphemes. Invalid
  rename input remains in the form with an inline error.
- Named workspace loading propagates the returned watcher command through both
  Commands and prompt dispatch. Tests verify the actual watcher receives the
  restored directory rather than merely updating desired-path bookkeeping.
- Metadata, Git, archive, remote and plugin results carry their originating
  workspace identity/generation. Late presentation cannot replace another
  workspace's viewer or consume a newer approval. Successful batch renames still
  reconcile global Undo and catalog paths after switching workspaces.
- Submitted directory copies, archive extraction and remote downloads have
  lifetimes independent of replaceable previews/listings. Explicit cancellation
  stays in Operations. Destination paths are captured at submission.
- Portable filenames use `/` for directories and `@` for links, retaining the
  marker when clipped. Rich icons are unchanged.
- Real Terminal interaction caught wrapping in fixed-width context menus.
  Menus now derive width from all labels; renderer and last-row hit tests cover
  all four reference sizes.

### Reproduced shutdown failure and repair

The expanded non-short race run in `race-full-nonshort-final.txt` **failed**:
navigation cleanup timed out draining two commands. The targeted old-behavior
repeat in `race-navigation-reproduction.txt` reproduced four failures. Do not
report either run as passing.

Cancellation-first harness cleanup subsequently exposed a production problem:
`race-navigation-shutdown-fixed.txt` timed out with a Git runner blocked in
`os/exec.Cmd.awaitGoroutines` on output pipes after cancellation. The Git runner
now sets a 500 ms `WaitDelay`, bounding pipe cleanup in addition to command
execution. Incomplete output remains an error, never a successful status result.
This does not claim to terminate arbitrary descendant process trees.

`git-pipe-regression-before.txt` demonstrates failure using an overlay with the
old unbounded wait; `git-pipe-regression-after.txt` passes with the production
fix. The shell-free helper releases its own descendant during cleanup.
`race-navigation-pipes-fixed.txt` passes ten repeated navigation, harness and
shutdown runs (99.837s). No deadlines were relaxed or assertions removed.

### Actual Windows Terminal checks

`native-acceptance-4` records the menu-corrected development build, SHA-256
`19BB23D12A44DDF5019440D3597DE80CBB84538F432F664BD480B6BAB99A8A70`.
Its isolated profile and fixture passed:

- Explicit portable setup, Enter/Backspace navigation, keyboard and mouse
  workspace creation/close, bounded folder search, reveal/resume, rename cancel.
- One injected wheel notch moving one file; narrow sidebar drawer reaching the
  final disk; wide-sidebar restoration.
- Inactive workspace context rename/close, empty-name inline validation and
  correction, and closing the final workspace into a fresh launch-folder view.

`portable.png`, `workspace-menu.txt`, `smoke-results.json`,
`interaction-results.json` and `workspace-actions-results.json` retain evidence.
Automation selectors were corrected to tolerate overlays and to send literal
spaces; the failed harness attempts are not application acceptance passes.
This build predates the Git pipe fix, which does not change the renderer.

`native-acceptance-3/workspace-resource-observation.json` records 100 real
Terminal create/close cycles after 12 warmups: RSS remained around 32 MB, handles
settled at 289 and threads at 18; the two-second idle observation consumed no
additional process CPU time. These are workspace-only observations, not a leak
proof, search/preview-cycle coverage, physical-input latency or all-platform
resource acceptance.

### Controlled panel comparison

`candidate-comparison-1/results.json` compares preserved Xelif `4e61438a` with a
Daily Driver panel snapshot on the same 2,000-entry corpus. Membership, order and
focused identities match; one warmup is excluded and 12 alternating samples
are retained per case.

| Internal panel workload | Preserved Xelif median | Development snapshot median |
|---|---:|---:|
| Navigation plus rendering | 1.406 ms/op | 1.467 ms/op |
| Forced directory reload | 2.234 ms/op | 2.251 ms/op |

This is not the final frozen candidate, whole-application timing, physical
input-to-paint p95 or native provider search. Do not mix samples from different
runs to rank it against Superfile. Everything has only a Session-0 service here;
`es.exe` is unavailable and the interactive IPC endpoint is not established.

### CI preparation and publication boundary

The retained Python workflows explicitly use `hotkeys-legacy.toml`; they do not
change new-profile defaults to satisfy legacy Ctrl+A/Escape inputs. Their CI
profile roots are isolated under runner temporary storage. A read-only doctor
check replaces killed first-run setup, and log collection uses the Xelif path.
Six harness unit tests pass (`python-unit-final.txt`). The Go CI repetition now
includes workspace and operation ownership regressions on all three native OSes.
Strict linter rules remain unchanged; its CI time budget is ten minutes to allow
cold analysis, not a reduction in checks.

ASVLCII authorized pushing a verified source checkpoint to `main` for CI. That
authorization does not publish a release/package or replace the installed app.
GitHub identity and remote main were rechecked; public npm latest remains 0.1.0,
and the latest GitHub release remains v0.1.2. The development source is not a new
release version. Final combined checks and hosted CI must still be recorded.

## Source publication checkpoint — 2026-11-02

After the inherited-pipe repair, the complete non-short suite passed
(`full-shutdown-fixed.txt`, root package 117.724s), followed by the complete short
native Windows race suite (`race-shutdown-fixed.txt`, root package 101.999s).
Vet passed (`vet-shutdown-fixed.txt`). Final doctor/Git/configuration component
race tests passed (`final-component-checks.txt`). Strict analysis completed with
exit zero and no findings (`strict-lint-resumed-publish.txt`); a preceding run
printed zero findings but then timed out and is **not** counted as a pass.

Module tidy passes after recording `go-isatty` as a direct import and normalizing
Go module file line endings (`module-tidy-lf-final.txt`). No dependency versions
or checksums changed; `.gitattributes` now preserves LF for both module files.
Launcher boundary tests passed with seven passes and one platform-specific skip
(`launcher-tests-final.txt`). Python harness unit tests passed (six tests).

`native-acceptance-6` is a fresh isolated build for the resumed Terminal checks.
Earlier fixtures, including the interrupted move test, remain preserved. These
checks are local Windows 11 build 22631/Windows Terminal evidence, not Windows 10,
classic console, macOS or Linux desktop acceptance. Hosted native CI and clean
six-target npm/Bun smoke checks remain to be inspected after the authorized push.

No installed-command replacement, live profile migration, font/Terminal setting
change, npm publication or GitHub release is part of this source checkpoint.

The resumed strict run completed successfully with zero findings. Fresh
`native-acceptance-6` (SHA-256
`F0D63BDC44A353AA30B5767A5656A01F4B741BC907D2C27A4648B5991DFA45E4`)
passed the core portable smoke checks. Its isolated file workflow then verified
cut/paste, Undo, copy with identical bytes, and Escape cancelling permanent
deletion without mutation (`file-workflow-results.json`). The first final-dialog
assertion expected different wording; the continuation rechecked the file bytes
and actual cancellation without repeating the mutations. Evidence remains
separate from failed harness attempts.

The confirmation correctly accepted Escape, but its hint still advertised the
quit binding. The source now advertises the cancel binding; a renderer/mouse
regression verifies it (`dialog-hint-final.txt`). The recorded Terminal binary
predates this display-only hint change; it is not a frozen release artifact.
