# npm and Bun preview packaging

The Go application is unchanged in architecture. A dependency-free JavaScript
launcher selects one exact-version native package, inherits the terminal,
arguments, working directory and environment, and returns the app's exit status.
Only the wrapper exposes the `xelif` command. Installation has no lifecycle
scripts or postinstall downloads. Config, hotkeys and themes are embedded by Go.

## Layout and targets

- `packaging/npm/package.json`: public `@asvlcii/xelif` launcher and exact optional dependencies.
- `packaging/npm/bin/xelif.cjs`: platform selection and process handoff.
- `packaging/npm/build.mjs`: native Go build and whitelisted staging directories.
- `packaging/npm/pack-smoke.mjs`: real tarballs, isolated npm/Bun global installs and command checks.
- `.github/workflows/npm-package-ci.yml`: native Windows, macOS and Linux checks on x64 and arm64; artifacts only, no automatic publication.

Native packages are `@asvlcii/xelif-{win32,darwin,linux}-{x64,arm64}`.
Their `os`/`cpu` metadata filters installation; missing optional dependencies
produce an actionable error, not a network download at startup. Versions must
match the launcher exactly. The build refuses a Go/npm version mismatch and an
existing output directory instead of deleting or replacing prior artifacts.

macOS builds use CGO and Foundation so Trash support is retained. Windows/Linux
use CGO-disabled builds. Native CLI/installation tests are not interactive TUI
verification on every architecture. macOS builds are not Apple-notarized and
Windows builds are not Authenticode signed. The documented development launcher
on the author's PC remains independent and is not overwritten by packaging.

## Build and check

Install the Go version declared in `go.mod`, Node.js 22+, npm and Bun. On macOS,
also install the Xcode command-line tools for Foundation/CGO.

From the repo root (choose new output paths outside the checkout if desired):

```sh
node --test packaging/npm/test/launcher.test.cjs
node packaging/npm/build.mjs /absolute/path/to/new-stage
node packaging/npm/pack-smoke.mjs /absolute/path/to/new-stage /absolute/path/to/new-smoke
```

The launcher tests compile only a disposable process fixture. They check argument
quoting, UTF-8, inherited stdin/stdout/stderr, cwd/environment and exit status.
POSIX signal forwarding is tested on Unix; the Windows signal test is explicitly
skipped rather than pretending Unix signals are Windows console input.

The pack smoke uses a loopback-only fixture registry with isolated caches,
prefixes and XDG directories. It serves the real current-platform tarball, rejects
foreign-platform downloads, inspects exact packed file lists, and checks npm's
global command, Bun's global command and the launcher under both runtimes. It
does not install globally into the user's normal Node/Bun installation, touch
their Xelif profile, or publish anything. `results.json` records the evidence.

## First publication

The name is proposed: a public registry 404 does not establish scope ownership.
The publisher must own the npm user/organization scope `asvlcii`, and separately
authenticate to npm. GitHub authentication is not npm authentication. Never paste
tokens, OTPs or passwords into issues, source files or chat.

1. Finish all six native package jobs and the ordinary application CI gates.
2. Download the `npm-*` artifacts from that exact successful commit. Keep the
   platform tarballs and the one shared launcher tarball; inspect their manifests
   and integrity before publishing. Do not substitute untested local builds.
3. Verify the npm account with `npm whoami --registry=https://registry.npmjs.org`.
   Verify ownership or write permission for the `asvlcii` scope before publishing.
4. Publish the six native tarballs first, each with
   `npm publish <native.tgz> --access public --tag preview`. Wait until all six
   exact versions are readable from the public registry. Publish the launcher
   tarball last with the same flags. Do not overwrite existing immutable versions
   or silently skip a conflicting package.
5. Repeat clean npm and Bun installations against the public registry before
   advertising the installation commands. A local fixture-registry smoke alone
   is not proof that registry publication works.

After publication, users install the preview with:

```sh
npm install -g @asvlcii/xelif@preview
# or
bun add -g @asvlcii/xelif@preview
```

Future releases may configure npm trusted publishing/OIDC per package, but no
credentials, npm secrets or automatic publishing workflow are introduced here.
The agreed future website is `asvlcii.com/xelif`; the package links to GitHub
until the domain and website are actually configured. No DNS changes are made.

References: [npm optional dependencies and platform filters](https://docs.npmjs.com/cli/v11/configuring-npm/package-json/),
[Bun global installation](https://bun.sh/docs/pm/cli/add),
[npm trusted publishing](https://docs.npmjs.com/trusted-publishers/).
