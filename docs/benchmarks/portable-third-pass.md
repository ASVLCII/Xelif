# Portable performance and extraction safety, third pass

Baseline: `aff5844a45c78f8137e9d0a338b5431cd56e8de2`.
Branch: `perf/portable-third-pass`.

## Shared scanner changes

The hash planner counts sizes before allocating candidate buckets. Unique sizes
need no per-size candidate allocations; repeated sizes share an exact-sized
backing array. The all-equal-size case reuses completed discovery storage. It
preserves size/path ordering, byte-budget reservation, warning ordering and
deterministic results.

Workers now grow buffers geometrically rather than reallocating for every
slightly larger file. The existing 128 KiB per-worker maximum is unchanged;
cumulative allocation decreases, but retained capacity can be almost twice the
current file's size below that maximum. Workers check cancellation before
claiming another job or allocating its buffer. Content hashing and before/after
snapshot checks are unchanged.

Native Windows/amd64, Go 1.26.10, Intel Core i5-12400F, GOMAXPROCS=4. Five 300 ms
samples, medians below. These isolate shared algorithms, excluding Windows
identity lookup. They do not measure whole-app speed or native macOS/Linux speed.

| Workload | aff5844 | Candidate | Allocated bytes per operation, before / after |
| --- | ---: | ---: | ---: |
| Plan 10,000 unique-size files | 1.277 ms | 0.587 ms | 2,037,593 / 591,480 |
| Plan 10,000 equal-size files | 2.408 ms | 1.357 ms | 1,798,754 / 0 |
| Plan 10,000 files in size pairs | 1.575 ms | 1.427 ms | 1,500,634 / 992,568 |
| Hash 1,024 gradually growing files | 42.045 ms | 38.205 ms | 9,758,398 / 925,904 |

The last case reduces cumulative allocated bytes by 90.5%; this is not a peak
RSS claim. Benchmark reference helpers retain the actual aff5844 planner and
worker allocation algorithms. Both sides use identical fixtures and sorting,
unchanged hashing, warmed metadata/cache, and checked digests.

```sh
go test ./src/internal/duplicates -run 'TestHashPlanningMatchesAff5844|TestHashWorker|TestScanWorkerBufferGrowthPreservesDigests' -count=1
go test ./src/internal/duplicates -run '^$' -bench '^BenchmarkHash(Planning|WorkerGrowth)$' -benchmem -cpu=4 -count=5 -benchtime=300ms
```

## Windows identity fallback

Unix keeps immediate device/inode indexing. Windows previously compared every
new identity against all previous FileInfo values. Small Windows scans still
use that strategy; beyond 1,024 unique identities the set promotes to a real
volume/file-ID index obtained through public handle APIs. Failed identity
queries retain SameFile lookup, including comparisons across indexed/fallback
entries in either direction. No private runtime fields or guessed metadata
identities are used.

A focused 8,192-identity benchmark measured approximately 991 ms before and
199 ms after, with small-set parity. Full scan measurements use a separate
export of aff5844 and the existing Scan benchmarks, alternating baseline and
candidate order for five rounds:

| Native Windows workload | aff5844 median | Candidate median | Bytes per operation, before / after |
| --- | ---: | ---: | ---: |
| Discover 1,000 distinct-size files | 63.07 ms | 61.75 ms | 2,047,734 / 1,929,394 |
| Discover 10,000 distinct-size files | 1,905.37 ms | 693.12 ms | 21,229,520 / 27,096,296 |
| Hash 1,000 empty duplicates | 69.94 ms | 70.34 ms | 2,980,665 / 2,900,889 |
| Hash 1,000 32-byte duplicates | 74.40 ms | 74.33 ms | 2,967,814 / 2,897,290 |

The large discovery workload is 2.75x faster, but allocates 27.6% more bytes and
33.4% more objects for handle/stat/index bookkeeping. Small duplicate scans show
no material speed gain. Measurements concern warm local scans, not cold disks,
network drives, Everything latency or frame rate.

```sh
go test ./src/internal/duplicates -run '^$' -bench 'BenchmarkScanDiscovery/files_(1000|10000)$|BenchmarkScanSmallDuplicates' -benchmem -cpu=4 -benchtime=300ms -count=5
go test ./src/internal/duplicates -run '^$' -bench '^BenchmarkWindowsIdentitySet$' -benchmem -benchtime=1x -count=3
```

## Confirmed extraction defects and repairs

Native Windows regressions first demonstrated that both ZIP and TAR full
extraction could overwrite a pre-existing sibling file outside the requested
destination and report success. The upstream extractor used a string-prefix
containment check. A pinned, MIT-licensed local copy of xtractr now enforces
relative-path validation plus os.Root output operations across the retained
format decoders, including directory creation, timestamps, CPIO links and CUE
output. Final destination symlinks/junctions are rejected; opened-root identity
is verified. The UI reserves a fresh destination exclusively.

Selected ZIP extraction now also uses rooted operations and pinned parent
handles. Replacement no longer deletes the existing target before rename, so
failed publication preserves it. Cancellation is checked before publication;
the new cancellation regression fails against aff5844 and passes after repair.
Cancellation cleanup uses the pinned parent rather than a mutable parent path.

See [the local dependency patch](../../third_party/xtractr/README.xelif.md) for
provenance and maintenance instructions. Scope is Xelif's ExtractFile flow with
SquashRoot disabled. Existing hard links retain filesystem semantics; initial
destination ancestors must be trusted. Unused queue/move/delete APIs, CUE input
reads, decompression quotas and full-extraction cancellation are outside this
output-containment repair. No claim that every dangerous bug is removed is made.

## Validation and limits

Feature and interaction parity are acceptance requirements. No archive decoder,
rendering path, input binding, refresh cadence or scrolling behavior was removed
to obtain these results. The same baseline/candidate render fixture produced
identical hashes for six selected/scrolled/narrow/wide frames and a 96-frame
matrix across Nerd Font, Unicode, plain, no-color, light-theme and neutral-icon
modes. Native navigation, mouse/wheel, tab, refresh and extraction regressions
passed in the final short suite. This is automated parity evidence, not a live
terminal smoothness measurement.

Native Windows: complete serialized short suite, explicit nested extractor
suite, vet, module verification, module-tidy check, Windows build and CLI version
smoke. Regressions cover 1,000 randomized planner comparisons, bounded budgets,
large sizes, buffer growth, cancellation, identity fallback transitions, file
replacement, archive path escape and native Windows junction containment.
Normal ZIP/TAR/tar.gz/gzip, RAR password fallback and ISO extraction also passed.
Windows tests requiring symbolic-link privilege skip; junction tests execute.

Cross-compilation only: application builds for Linux and macOS on amd64/arm64,
plus Windows/arm64; all package test binaries and the nested extractor test
binary for Linux/amd64 and macOS/arm64. None were executed on those systems.
CI now explicitly includes the nested extractor suite in its native OS matrix;
that remote CI run has not been triggered here. Interactive UI, Python UI
automation and golangci-lint were not run in this pass.

DeepSeek V4.1 Flash Free and GLM-5.3-Flash Free were selected at their catalog's
maximum `max` variant. Both failed with HTTP 429 free quota exhausted, cost 0.
Native Codex agents performed implementation/review under the user's subsequent
authorization. No paid OpenCode models were invoked.
