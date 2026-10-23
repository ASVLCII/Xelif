# Xelif extraction boundary patch

This is the Go source of `golift.io/xtractr v0.3.2`, copied from the Go module
cache without changing the upstream format decoders. The original MIT license
is retained in `LICENSE`. Xelif selects this module with a local `replace` in its
root `go.mod`; no download from a different fork is required.

Upstream module checksum (recorded in the `aff5844` baseline's `go.sum`):
`h1:WquFJg9Gdj8aRMrJsBPrVYtI0Cp9Qw7zkIW9EJZXhBY=`.
The small RAR fixture in `test_data/archive.rar` also comes from that module.

The upstream ZIP and TAR extractors accepted sibling paths sharing the output
directory's string prefix. A ZIP or TAR containing
`../output-private/keep.txt`, extracted to `output`, could overwrite the existing
sibling file and return success. This was reproduced on native Windows before
applying the fix.

Local changes:

* `ExtractFile` opens one `os.Root` for the destination and retains it for the
  operation, including format fallback and password retries.
  Root acquisition rejects a final symlink/junction and compares the inspected
  directory's identity with the opened handle before extracting. Destination
  ancestors must remain trusted during acquisition; this is not a claim that
  root selection resists an attacker controlling the entire parent directory.
* `containment.go` validates destination-relative paths with `filepath.IsLocal`
  and performs output opens, directory creation, timestamps, CPIO symlinks, and
  CUE writes through that root. This closes check/use races involving escaping
  symlinks and Windows junctions; the boundary is enforced during resolution.
* The existing long-file-name fallback uses rooted checks and exclusive creation.
* External CPIO symlink targets are rejected before creation. Links staying
  within the extraction root retain upstream behavior.
* Original format decoders, extension/signature fallback, ordinary archive permissions,
  and archive timestamps are retained.

Run the added tests **from Xelif's module root** so the existing selected
dependencies and checksums are used:

```sh
go test -count=1 golift.io/xtractr
```

The upstream queue-management/move/delete APIs are retained for source
compatibility; Xelif uses only `ExtractFile` with `SquashRoot` disabled. Those
general-purpose queue APIs and CUE input reads are outside this extraction-output
boundary patch. Resource/decompression quotas and cancellation are unchanged.
Existing hard links inside a user-supplied nonempty destination retain their
ordinary filesystem semantics; Xelif's full-extraction UI chooses a fresh
destination directory.
Rooted file/directory creation uses ordinary permission bits only; special
setuid/setgid/sticky creation bits are not imported from untrusted archives.

When updating xtractr, reapply these narrow I/O changes and run the regression
suite explicitly: nested-module tests are not included in `go test ./...`.
