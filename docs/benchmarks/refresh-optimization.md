# Input-path refresh optimization

Date: 2026-10-22. Baseline means Xelif immediately before this refresh change,
not upstream superfile or the earlier lint cleanup.

## Scope

This change removes repeated filesystem work from ordinary keyboard, mouse,
blink and progress events. It does not replace Everything, change search
limits, redesign the UI, or add a persistent file index.

- Sidebar input updates reuse the last displayed snapshot for one second.
  Query changes, pin toggles, recent-folder additions, alias renames and
  explicit refreshes bypass that gate. The original loaders and their cleanup
  behavior are retained, including discovery of sections that are not displayed.
- Focused file panels have a minimum one-second fallback interval instead of
  rereading folders with fewer than 100 entries after every event. Larger
  folders retain the existing two-to-three-second intervals; unfocused panels
  retain their three-second interval. Empty and no-match results are bounded too.
- A panel records the inputs used for its last load. Location, search text,
  sorting, reverse order, dot-file visibility and case-sensitive sorting changes
  bypass the interval immediately. Target-file positioning and forced refreshes
  remain immediate.
- Existing filesystem notifications still force refreshes. A recurring
  one-second tick keeps fallback polling alive while the application is idle.
- Completed file operations, rename, shell/editor/plugin/Git commands and partial
  mutation failures reconcile the affected views without waiting for polling.
  Newer operation handlers that already refresh panels on success are not forced
  to read them a second time merely to reconcile sidebar paths.

The intervals are polling eligibility thresholds, not hard freshness deadlines.
An intervening input/operation refresh can make the next periodic tick too early;
the following tick then performs the poll. Scheduling, synchronous I/O and event
queue load can add latency. Notifications and explicit mutation completions do
not wait for this fallback. No claim of instantaneous external-change detection
is made when the watcher is unavailable.

## Regression coverage

New tests cover refresh boundaries, empty/no-match snapshots, query changes and
clearing, navigation, target positioning, all ordering inputs, selection anchors,
forced refreshes, and external creation/deletion reconciled without a watcher.
Sidebar tests cover immediate internal invalidation, external pinned-file edits,
disabled sidebars, explicit bypass, and independent wheel viewport preservation.
Root tests cover the recurring tick and completed/partially failed operations.

The compression-selection event-loop test now waits for a next-message rendezvous
after its completion signal. The signal is observed by Bubble Tea's filter,
before model.Update applies the completed operation. Waiting for command/file
completion alone is therefore insufficient before reading refreshed UI state.
No production timeout or test timeout was increased.

## Benchmark method and limitations

The retained corpus has 50-file and 2,000-file directories, ASCII/Unicode names,
mixed extensions, ten recent directories and four pinned directories. Home and
recent/pinned state are isolated; actual host disk enumeration is retained.

The same fixture builds baseline and candidate binaries. A six-file Go overlay
substitutes saved pre-change production sources for the baseline; other source
and dependencies are shared. This is a focused comparison, not reconstruction of
an old release. Correctness checks compare sixteen panel/sidebar SHA256 outputs
across both folder sizes and section configurations, including forced reloads.

Measurements alternate baseline-first and candidate-first order, exclude a
warm-up pair, and run without concurrent build/test jobs:

- Twelve paired 500 ms samples measure the cached navigation input path, with
  and without direct file-panel/sidebar rendering. These samples are shorter
  than the sidebar polling interval and must not be described as total refresh
  cost or whole-application speed.
- Three paired two-second update-only samples include periodic refresh
  eligibility during sustained input.
- Twelve paired forced-reload samples measure the work that remains necessary
  when data really must be reread.

The benchmark calls synchronous root Update and optionally renders the file panel
and sidebar. It does not start the event loop, execute returned commands, include
the root compositor/footer, or measure terminal paint, idle power, startup,
Everything, preview, metadata, Git or watcher latency. Disk enumeration, storage
caching and OS scheduling are host-dependent. Ratios are descriptive local
measurements, not statistical guarantees or predictions for every workload.

Runner, raw logs, hashes and summaries:
`X:\Xelif-work\benchmarks\refresh-root-update-20261022`.

## Verification notes

The full lint configuration reports zero issues. The pre-change source snapshot
contains 416 Go files; the preservation audit found changes only in the seven
intended existing source/test files. New test/helper files are additional.

One full-suite run hit the existing Windows `open $(echo "~")` shell-substitution
test. Three isolated repetitions passed on both baseline and candidate; that
does not erase the observed intermittent failure. The source already describes
Windows command substitution as flaky. No shell-substitution behavior was changed.

The complete short suite passed on rerun with no skipped failing test:
`go test -p=1 -short -count=1 -timeout=5m ./...`.
Log: `X:\Xelif-work\refresh-verification-rerun.log`.
Configured lint (zero issues), `go vet ./...`, Windows/amd64 build, Linux/amd64
and macOS/arm64 cross-builds also passed. Cross-builds are not native runtime
verification on Linux or macOS. No interactive terminal-paint or drive-unplug
acceptance test is claimed.

Preservation audit: `X:\Xelif-work\refresh-source-preservation.json`.
Final lint report: `X:\Xelif-work\refresh-lint-final.json`.

## Results

Median synchronous navigation plus file-panel/sidebar rendering, with all
sidebar sections displayed (twelve paired 500 ms rounds):

| Folder corpus | Baseline | Candidate | Reduction in median time |
| --- | ---: | ---: | ---: |
| 50 files | 3.430708 ms | 2.237290 ms | 34.79% |
| 2,000 files | 3.266397 ms | 2.231978 ms | 31.67% |

These reductions use the ratio of the separately computed medians. Raw summaries
also report the median of paired old/new ratios; those are a different statistic.
The benefit is avoided routine I/O, not a demonstrated faster sort, search engine,
disk or terminal. Parent-side deterministic calculations independently checked
these conversions and percentages.

Update-only hot-path medians fell from approximately 0.94–1.07 ms to 1.60–1.63 us.
Those large ratios must not be used as whole-application speed claims. The
three-pair two-second update-only check crossed the refresh interval under
continuous input and retained the same general reduction; it is not a tail-latency
or idle-event-loop measurement.

The balanced twelve-pair forced-reload check found paired median ratios of
approximately 1.00–1.02 old/new across its four cases. There is no clear forced
reload improvement to claim. An earlier three-pair run included a roughly 6%
slow case that did not persist in the balanced rerun; both raw runs are retained.
Forced reloads add two allocations (about 16 bytes) per operation in the candidate;
routine navigation/rendering uses fewer allocations. This is not a universal
zero-overhead or statistical-equivalence claim.

Raw result directories:

- `refresh-cached-hot-path-20261022-095154` (twelve paired navigation rounds)
- `refresh-long-cadence-20261022-095413` (three paired two-second rounds)
- `refresh-forced-reload-20261022-095640` (final twelve-pair forced reload)
- `refresh-forced-reload-20261022-095523` (retained initial three-pair probe)

Each directory contains `refresh-summary.json`, raw samples, per-pass logs and
run metadata. Corpus preparation, source/binary hashes, parity output and the
Windows prompt diagnostic logs are in their parent directory.

## Installed build

Installed executable: `X:\Xelif\bin\spf.exe`.
SHA256: `12F57E41EF70D1F0FDD37BB7305770582117FCB1E7942FAE86A24DD341660EF3`.

Previous executable retained at
`X:\Xelif-work\spf-before-refresh-20261022-095851.exe`.
Installation hash/backup record: `X:\Xelif-work\refresh-installed.json`.
No commits or publication were made.
