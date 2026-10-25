# Xelif v0.1.1

This release adds the opt-in [operation center](operation-center.md). Press
**Alt+O**, choose **Operations** in Commands, or click the Processes footer.
Its visibility is remembered; task history stays session-only.

## Changes

- Operations and Background tabs, filtering, task details, safe cancellation
  requests and bounded finished-task history, with mouse and keyboard controls.
- Bounded progress delivery and pooled copy buffers, without counting an entire
  directory tree solely to show a percentage.
- Staged large copies and ZIP creation, extraction cancellation checkpoints,
  and source retention until cross-volume copying completes successfully.
- A shutdown fix: workers waiting behind a file-error dialog stop waiting when
  Xelif shuts down, rather than preventing worker cleanup from completing.
- Compact, centered welcome text with upstream attribution.

Cancellation is cooperative, not rollback. Completed changes remain; an active
filesystem call may need to finish before cancellation takes effect. Closing the
operation center does not cancel work. See the operation-center guide for limits.

## Install or update

```sh
npm install -g @asvlcii/xelif@0.1.1
# or
bun add -g @asvlcii/xelif@0.1.1
```

Then run `xelif`. Both package managers use the same npm packages. Node.js 22+
is required by the global launcher; keep optional dependencies enabled.
Native packages cover Windows, macOS and Linux on x64 and ARM64.

## Release gates

Publish only the tarballs from the successful native package CI run for the
release commit. Check all six native installation jobs, ordinary application
tests, native race suites, and the explicit extraction-boundary tests. Recheck
artifact manifests, CPU headers, exact file lists and integrity before upload.
Publish the six native packages before the launcher, then verify clean npm and
Bun installations against the public registry. Attach those exact tarballs and
their checksums to the GitHub release.

Targeted Windows race testing includes repeated task/operation regressions and
the queued-file-processor shutdown regression. These checks do not prove
interactive behavior on every platform or Everything-level search performance.
Windows executables are unsigned and macOS builds are not notarized.
