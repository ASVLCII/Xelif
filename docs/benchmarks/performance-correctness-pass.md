# File operations, search, previews, and resource use — 2026-10-23

This pass fixes reproducible correctness problems and removes unnecessary work
from directory loading, duplicate scanning, local search, and previews. It builds
on the separate [frame-local column rendering pass](frame-local-columns.md).

## Correctness fixes

- Paste validation now rejects descendants named `..nested`. Previously the
  ancestry check confused that filename with the `..` parent component, allowing
  a recursive copy destination inside its own source.
- Read-only directory trees are copied with temporary owner access, then their
  original permissions are restored after descendants are complete. This also
  fixes the copy stage of move/undo fallback across filesystems.
- A destination directory appearing during a copy walk now causes an error.
  The old walk could rename directory creation while still directing its
  descendants to the original path, creating inconsistent or merged results.
- FIFOs and other unsupported special files return `errors.ErrUnsupported`
  before opening. Previously copying a FIFO could wait indefinitely for a writer.
- The legacy `pasteDir` cut route now uses the existing no-clobber move helper,
  including cross-device copy fallback. The normal clipboard cut route already
  used that helper; its real cross-device behavior was validated without being
  attributed as a new fix.
- Overlapping duplicate-scan roots no longer report one pathname as its own
  duplicate when hard links are included. Distinct hard-link paths still count
  according to the user's option.
- A directory refresh preserves the focused pathname when earlier rows are
  inserted or removed. Previously the same numeric cursor index could silently
  point to a different file before the user's next open or delete action.
  Explicit target requests and directory/query changes keep their own rules.
- Descending sort comparators now treat equal keys as equal. Negating the old
  less-than result made an entry sort before itself and violated Go's sorting
  contract for equal sizes, dates, names, natural names, and types.
- Caches enforce expiration on lookup, avoid evicting unrelated entries when
  updating an existing key, and maintain capacity even for an empty-string key.
  They no longer create cleanup goroutines that retain discarded caches forever.
- Image and thumbnail caches include source size and modification time, so
  edits invalidate old results and deleted sources produce errors. Concurrent
  thumbnail requests for one revision share a conversion; a late older
  conversion cannot replace a newer cached revision. Returned thumbnail files
  remain valid until generator cleanup.

## Performance changes

- Directory loading fetches metadata once per visible candidate instead of twice.
  Folder filtering rejects unmatched names before fetching metadata. A Linux
  syscall trace of one selective 2,000-file fixture recorded 2,069 baseline
  versus 69 candidate `newfstatat` calls, including identical test setup overhead.
- Duplicate discovery uses device/inode identity indexing on Unix, replacing
  quadratic comparisons against previously seen files. Other platforms retain
  the existing `os.SameFile` fallback. Each hashing worker reuses a lazily sized
  buffer of at most 128 KiB instead of allocating 128 KiB per file.
- Selective local searches reject basenames before metadata requests while
  preserving traversal into nonmatching directories, visit limits, cancellation,
  and errors on matching paths.
- Directory previews use batched unsorted reads and a bounded heap to select
  the first visible rows in directories-first/name order. They retain one batch
  plus the viewport prefix instead of the entire directory, and avoid a complete
  sort. Enumeration still visits every entry to determine the correct prefix.

## Paired local benchmarks

Linux/amd64, AMD EPYC 9V74, Go 1.26.10, GOMAXPROCS=4. Each case had one
discarded warmup pair and five measured baseline/candidate pairs, alternating
execution order with 500 ms benchmark windows. No other build or test jobs ran
during comparison. Fixture creation was outside the benchmark timer.

| Workload | Baseline | Candidate | Ratio | Allocated bytes before → after |
| --- | ---: | ---: | ---: | ---: |
| Load 2,000-file folder | 7.036 ms | 4.476 ms | 1.57× | 2,010,392 → 1,273,693 |
| Filter that folder to one filename | 5.650 ms | 2.209 ms | 2.56× | 6,075,686 → 5,337,853 |
| Discover 10,000 duplicate candidates, distinct sizes | 116.357 ms | 29.085 ms | 4.00× | 9,490,414 → 9,697,449 |
| Scan 1,000 identical 32-byte files | 29.058 ms | 3.915 ms | 7.42× | 132,713,844 → 1,711,703 |
| Selective local search, 10,000 files / 10 matches | 27.397 ms | 7.343 ms | 3.73× | 6,516,753 → 2,515,434 |
| Render 24 preview rows from 10,000 entries | 6.994 ms | 3.029 ms | 2.31× | 1,538,541 → 1,221,224 |

Values are separately computed medians. Duplicate indexing trades a small map
allocation increase for better scaling: discovery-only allocation rose 2.2%
in the 10,000-file case. Tiny-file duplicate scans allocate 98.7% fewer bytes.
Directory previews allocate 20.6% fewer bytes but make about 1.6% more allocation
calls because of batched reads. These are workload-specific measurements, not
whole-application speed, retained RSS, power, or native Windows/macOS results.

The thumbnail concurrency regression deterministically verifies that 16
overlapping requests invoke one converter, with one additional conversion after
a source edit. The cache lifecycle regression verifies that a discarded cache
can be garbage collected rather than retained by a background worker.

## Reproduction

The new benchmarks live beside their respective tests. From the repository root:

```sh
go test ./src/internal/ui/filepanel -run '^$' -bench '^BenchmarkDirectoryLoad$' -benchmem
go test ./src/internal/duplicates -run '^$' -bench '^BenchmarkScan(Discovery|SmallDuplicates)$' -benchmem
go test ./src/internal/ui/globalsearch -run '^$' -bench '^BenchmarkLocalSearchSelectiveName$' -benchmem
go test ./src/internal/ui/preview -run '^$' -bench '^BenchmarkDirectoryPreview$' -benchmem
```

Baseline binaries use the same new benchmark tests and dependencies with Go
overlays restoring the relevant pre-change production sources from commit
`734be6ec6ecf691e5592b8711c6dad32105eb11f`. The earlier renderer optimization is
held constant in these comparisons. Saved binaries, overlays, per-pair output,
syscall traces, measurement script, and JSON summaries are under
`/workspace/.cache/perf` and `/workspace/.cache` in the prepared environment.

Source revision checks use size/mtime instead of hashing every preview request.
Edits that deliberately preserve both are outside that invalidation guarantee.
External thumbnail programs are mocked for deterministic concurrency tests;
native terminal paint and external converter throughput are not benchmarked.

## Validation

- Full fresh Go suite: 35 packages passed; 2,865 test/subtest passes, seven
  explicit skips, no failures. Ten additional packages contain no tests.
  Skips are existing opt-in corpus/benchmark setup and Windows-only cases.
- Race detector: file panels, duplicate scans, local search, caches, image and
  thumbnail previews, directory previews, metadata, and focused copy regressions
  passed.
- `go vet ./...`, the Linux application build, and all 12 existing tmux
  integration tests passed.
- The repository's configured golangci-lint v2.12.2 run reports zero issues.
- Windows/amd64 and macOS/arm64 application cross-builds passed. These are compile
  checks, not native acceptance tests.
- Real Linux cross-filesystem tests between `/tmp` and `/workspace` executed
  successfully for files, directory trees, and dangling symlinks; they were not
  skipped in this environment.
- Regression tests for recursion, read-only trees, destination collisions, FIFOs,
  self-duplicates, strict sorting, and refresh focus were exercised against old
  source snapshots to confirm the original failures.
