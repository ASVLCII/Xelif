# Xelif v0.1.0

The first stable Xelif release, including the three performance passes and
the native race-detector fix. This release uses the normal GitHub release
channel and npm's latest tag. The older preview remains available separately.

## Changes since the preview

- Lower allocation costs in navigation, rendering, file previews and duplicate hashing.
- Faster content search and large duplicate scans.
- ZIP/TAR extraction containment, including Windows junction checks.
- File-operation, sort-ordering, cache-lifetime and focus-preservation fixes.
- Synchronized integration tests and immutable background-command inputs.

## Release process

Version metadata and all six native optional dependencies use 0.1.0. Native
package CI builds and smoke-tests each supported OS/architecture. Publish those
exact artifacts, six native packages first and the launcher last, under latest.
Attach the same tarballs and SHA256SUMS.txt to the stable GitHub release.

The inherited upstream WinGet publication workflow was removed: its package
identifier targeted yorukot.superfile, not Xelif. No WinGet submission is part
of this release.

## Verification scope

The release is based on fix/native-race-detector at 8b8e86d. Its Windows normal
and race suites, shuffled root race run and repeated snapshot/helper regressions
passed locally. Fresh release CI results supply build/package validation for
the release commit. Interactive acceptance on every OS/architecture and the
Everything-provider speed comparison remain separate from those checks.

Windows executables are not Authenticode signed; macOS builds are not notarized.
Node.js 22+ is required for the npm/Bun launcher. Existing superfile-compatible
configuration paths and upstream notices are preserved.
