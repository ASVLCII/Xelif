# Search provider correctness and latency experiment

The opt-in same-package test `TestSearchProviderBenchmark` calls
`SearchLocalQuery`, `searchEverythingDirect` and, when `es.exe` is on PATH,
`searchEverything` directly. There is no fallback and no production command or
export. Ordinary `go test` skips corpus generation and live provider work.

Run from the repository with an existing Go 1.26 toolchain and dependencies:

```powershell
$env:XELIF_BENCH_SEARCH = '1'
$env:XELIF_BENCH_ROOT = 'X:\Xelif-work\benchmarks'
go test ./src/internal/ui/globalsearch -run '^TestSearchProviderBenchmark$' -count=1 -timeout=30m -v
Remove-Item Env:XELIF_BENCH_SEARCH
```

Set `TEMP`, `TMP`, `APPDATA`, `LOCALAPPDATA`, and the `XDG_*_HOME` directories to
an isolated run directory if running broader repository tests. Repository test
initialization can remove `spf_testdir` inside the temporary directory. Do not
run other builds, scans, or benchmarks during timing.

Each invocation creates and retains a new nonce directory. It never deletes a
previous corpus. There are exactly 10,000 empty regular files in 100 directories.
The fixed seed is 20261018; an explicit integer mixer produces deterministic
filenames modulo the random isolation nonce. `manifest.json` records that nonce,
all relative paths and directory flags, and literal queries. The complete plan
is written before materialization. Eight bounded workers create empty files with
exclusive creation and error propagation. `setup.json` records creating/complete
status and final counts; a manifest alone does not prove corpus completion. Phase
and every-1,000-file progress logs are outside timed calls. The manifest and setup
record are additional control files, excluded from expected search results because
their basenames contain none of the query literals. No symlinks are created.

Queries exercise many matches (9,994), few (3), zero, ASCII case folding (1),
literal regex punctuation (1), and folder basenames (100). Unicode (1) is an
untimed correctness probe. Every query includes the unique nonce, so a global
provider can return the same complete result set as the scoped local walker.
The expected results are computed solely from manifest basenames with
case-insensitive literal substring matching. Directory matches must carry the
correct directory flag; child files do not inherit a basename match from their
parent folder. Paths normalize separators but preserve filename case.

Both result and local visit caps are 20,000, above the whole corpus and below
50,000. Any error, cancellation, partial response, reached/unsafe cap, duplicate,
missing result, extra result, path outside the corpus, or wrong directory flag
rejects the observation. Unit test `TestSearchBenchmarkValidator` exercises
those rejection paths. No mismatched result counts can produce a speed claim.

The helper does not install, start, configure, or reindex Everything. Missing
IPC service or absent `es.exe` is explicitly unavailable. A running provider
must pass two consecutive exact-set passes over every query inside a shared
30-second readiness period. An optional provider that fails to become ready is excluded with an explicit
incomplete-index/readiness reason and invalidates comparison, while valid local
samples can still be reported. Local readiness failure fails the test.
An already-canceled request must return cancellation and no results. A query
that loses correctness during warmup or measurement fails the test and leaves
`ComparisonValid` false. Local-only results are supported, but never constitute
a comparison against Everything. A successful test alone does not establish
that multiple providers were measured: inspect `Status` and `ComparisonValid`.

The first invocation is recorded separately before readiness. It may reflect
index lag and is excluded from all distributions; it is not a cold-cache claim.
Corpus generation, manifest construction, readiness, and validation are outside
the measured region. After two warmups per query and provider, each query gets
30 samples per available provider with rotating provider order. Only the actual
provider function call is timed. Each result set is validated after its timer
stops. Context creation is outside timing; local directory traversal and metadata
reads, IPC round trips, and CLI process startup/output parsing are inside their
respective provider timings. Unicode/cancellation are separate from timing.

`results.json` beside the manifest records every raw nanosecond observation,
phase, count, validation outcome, error, provider status, correctness probes,
Go version, test executable, CLI executable, OS, architecture, Windows CPU
identifier, corpus root and limits. Summaries use the median (mean of sorted
positions 15 and 16) and nearest-rank p95 (position 29 of 30). No caches are
flushed, and the tiny-file corpus measures name search rather than content or
general filesystem workloads. Keep any external watchdog at least 30 minutes
to match the Go test timeout; setup can be slow on this machine. Results describe this machine and index state;
they do not establish a universal provider ranking.


Initial implementation validation on 2026-10-18: the full `globalsearch` package,
including validator rejection tests, passed; the opt-in benchmark skipped in
that normal run. The first live attempt was interrupted during sequential setup
after 497.562 seconds, leaving 7,790 files and 78 directories at
`X:\Xelif-work\benchmarks\xb7361283df6ca16f4`. No provider timing occurred and
that incomplete corpus is retained. This setup bottleneck motivated the bounded
pool and empty files described above; it is not a provider performance result.


Final local-only run on 2026-10-18 passed with the revised setup (10.031 seconds
including Go invocation; test 8.15 seconds). All 10,000 empty fixture files and 100
directories exactly matched the UTF-8 manifest. Local readiness, Unicode and
pre-cancellation checks passed; all 180 measured samples and 12 warmups validated.
Everything IPC was called once and reported `Everything is not running`; es.exe
was absent from PATH and was not invoked. `ComparisonValid=false`; these results
cannot rank local search against Everything.

| Query | Complete matches | Samples | Median ms | p95 ms |
|---|---:|---:|---:|---:|
| many | 9,994 | 30 | 14.356 | 18.584 |
| few | 3 | 30 | 12.883 | 15.902 |
| zero | 0 | 30 | 13.309 | 16.492 |
| ascii-case | 1 | 30 | 13.797 | 15.707 |
| punctuation | 1 | 30 | 13.351 | 15.680 |
| folders | 100 | 30 | 12.603 | 14.599 |

The first local invocation was 548.533ms and is excluded from the table; no cache
flushing was performed. Raw results and manifest are retained at
`X:\Xelif-work\benchmarks\xb3c1090d8c47c71de`. Afterward, `identity.json` records
source/provider and Go executable SHA256. The temporary test executable's path
is recorded; Go removed that binary after the run, so its byte hash is unavailable.
The independent artifact/count/summary check is retained in the run's
`evidence/xelif/x2-report-validation.json`. The complete fixture has 10,000 files;
root-level manifest/setup/results/identity JSON are additional control artifacts.
