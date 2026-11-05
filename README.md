# Xelif

A terminal file manager with mouse and keyboard access. Inspired by [superfile](https://github.com/yorukot/superfile), developed by [ASVLCII](https://github.com/ASVLCII).

[Demo](#demo) Â· [Install](#install) Â· [Build](#build-from-source) Â· [Controls](#getting-around) Â· [Feature guide](website/src/content/docs/features.md) Â· [Verification](docs/completion-audit.md)

![Xelif browsing a disposable sample workspace with a monochrome interface, colored file-type icons, sidebar, and aligned file columns](docs/media/overview.png)

## Demo

### Daily Driver development checkpoint (unreleased)

`main` now includes a full-width workspace strip, independent workspace state,
one default file pane, portable/rich appearance setup, Xelif-owned profiles with
explicit legacy import, and indexed search adapters for Everything, Spotlight,
and installed `plocate`. Folder search remains a separate explicit scope.

This work is **not a new stable release**. The published downloads and demos
below describe the earlier release. See the [Daily Driver checkpoint and open
acceptance gates](docs/daily-driver-progress.md) before trying the development
source. Native desktop acceptance and Everything-equivalent speed are not yet
confirmed across all three operating systems.

Browse files, move through individual rows, filter a folder, select a range, open Commands, and reveal the information panels.

<details>
<summary>Watch the walkthrough</summary>

![Animated walkthrough of Xelif's actual rendered interface](docs/media/walkthrough.gif)

</details>

<details>
<summary>See the command palette</summary>

![Xelif's command palette over the file browser](docs/media/commands.png)

</details>

These are reproducible frames from Xelif's real TUI renderer, using disposable sample filesâ€”not a generated mockup or an OS terminal recording. The static screenshot is the default; animation is optional. [Demo source and reproduction](docs/media/README.md).

## What is different

| Area | Xelif |
| --- | --- |
| Navigation | Clickable breadcrumbs, tabs, sidebar folders, columns, Search, and Commands; keyboard alternatives remain available. |
| Presentation | Customizable monochrome default, recognizable color-coded file-type icons, compact controls, optional information panels. |
| Search | Direct Everything integration on Windows, advanced filters, saved searches, and bounded local fallbacks. |
| Workspaces | Multiple panels, independent tabs and history, session restoration, and named workspaces. |
| File tools | Range selection, batch-rename previews, ZIP browsing, duplicate detection, directory comparison, and copy-missing previews. |

Also includes Git status/actions, tags and collections, metadata preview/restore, declarative command plugins, and read-only rclone browsing with explicit downloads. See the [feature guide](website/src/content/docs/features.md) for prerequisites, commands, and safety boundaries.

Click **Sort** beside Search and Commands for an anchored dropdown: Name, Date
modified, Type, Size, or Natural, with separate Ascending and Descending choices.
The active choices are marked with dots. Use Up/Down and Enter, or click a row;
Escape or a click outside dismisses it. Sorting keeps the same item focused.

![Xelif Sort dropdown rendered by the application](docs/media/sort-menu.png)

Undo records the filesystem identity of each moved item. If a path is replaced
after the undo journal is recorded, Undo refuses to move that replacement. Edits
to the same file remain undoable. Identity checks and the later move are separate
operations, so they do not eliminate every concurrent filesystem race.

## Install

**GitHub release: `0.1.2` — Sort button.** Download a native archive from
[GitHub Releases](https://github.com/ASVLCII/Xelif/releases/tag/v0.1.2), extract it,
and run `xelif` (`xelif.exe` on Windows).

The npm `latest` release is currently `0.1.0`; it does not include the new Sort
button. To install that published
[`@asvlcii/xelif`](https://www.npmjs.com/package/@asvlcii/xelif) package with
either npm or Bun. Native executables are available for Windows, macOS and Linux
on x64 and ARM64.

```sh
npm install -g @asvlcii/xelif@latest
# or
bun add -g @asvlcii/xelif@latest
```

Then launch it from any directory:

```sh
xelif
# Or open a particular folder:
xelif "/path/to/a folder"
```

Install Node.js 22+ for the global launcher, including when installing with Bun.
Keep optional dependencies enabled: they select the executable for your machine.
A Go compiler is not needed. A Nerd Font is recommended for file-type icons.

The package uses exact-version optional dependencies, no postinstall scripts or
downloads, and a small Node.js 22+ launcher for the existing Go application.
[Packaging and verification](docs/npm-packaging.md). Running the same install
command again updates to the current stable release. If another Xelif installation opens,
check `Get-Command xelif -All` in PowerShell or `command -v xelif` on Unix.

## Build from source

Build from source if you prefer not to use the package above. Existing superfile installers and package-manager entries install upstream superfile, not Xelif.

Requires Go 1.26 or later.

```sh
git clone https://github.com/ASVLCII/Xelif.git
cd Xelif
go build -o bin/xelif .
./bin/xelif
```

On Windows, use these build/run commands instead:

```powershell
go build -o bin/xelif.exe .
.\bin\xelif.exe
```

A Nerd Font is recommended for the illustrated icons; the demo uses CaskaydiaMono Nerd Font Mono. Windows Terminal is the primary development environment. Everything must be installed and running for its native Windows search provider; local search remains available without it.

Development builds retain superfile's Go module path and `spf` compatibility,
but use independent `xelif` directories under the normal OS/XDG configuration,
data, state and cache roots. Existing configuration-file overrides remain
supported. **Appearance and setup** offers an explicit, validated copy/import of
supported Superfile preferences; it never moves or edits the source profile.
Use `xelif doctor --json` to inspect the actual paths and selected appearance.

## Getting around

| Action | Mouse | Default keyboard |
| --- | --- | --- |
| Open an item | Double-click | `Enter` / `Right` |
| Parent folder | Click a breadcrumb | `Left` / `Backspace` |
| Filter this folder | Click the search field | `/` |
| Search everywhere | Click **Search** | `Ctrl+G` |
| Command palette | Click **Commands** | `Ctrl+Shift+P` / `Alt+P` |
| Favourite a folder | Right-click a folder or sidebar entry â†’ **Toggle favourite** | `Shift+P`; sidebar context menu: `Shift+F10` |
| Information panels | Use the footer command | `Shift+F` |
| Range selection | Selection controls | `Shift+Space` |
| New / close workspace | Workspace strip / tab context menu | `Ctrl+T` / `Ctrl+W` |
| Next / previous workspace | Click a tab or use **Workspaces** | `Ctrl+Right` / `Ctrl+Left`; also `Ctrl+Tab` / `Ctrl+Shift+Tab` |
| Focus sidebar / adjacent file pane | Click the pane | `Shift+Left` / `Shift+Right` |
| Rename / undo rename or move | File actions / Commands | `F2` / `Ctrl+Z` |
| Cycle focus / quit | Commands | `F6` / `Ctrl+Q` |

These development defaults apply to new profiles. Existing custom bindings are
preserved; `hotkeys-legacy.toml` is available as an explicit alternative. Escape
closes the current edit/overlay rather than quitting in the familiar keymap.
Some terminals reserve shortcuts; Commands and the Workspaces picker provide
keyboard alternatives. Mouse-wheel handling advances one row per delivered
event; physical wheels and terminals can emit multiple events.

Shift+Arrow pane navigation follows screen order and stops at the outer edges.
At narrow widths, Shift+Left from the first file pane opens the sidebar drawer.
Older folder tabs inside a pane retain their picker and commands; their new-profile
shortcuts are `Alt+Shift+Left` / `Alt+Shift+Right`.

Right-click empty space for **New → New file / New folder**, Paste, Sort, View,
Refresh, and folder favourites. Right-click a file or folder for its actions;
folders also offer **Paste into this folder** and **Open in new workspace**.
Use **Shift+F10** for the same menu from the keyboard, including in empty folders.
Submenus support Enter/Right to open and Escape/Left to return. Existing custom
navigation bindings retain precedence. Properties shows the listed metadata
snapshot; it does not recursively scan a folder or calculate its total size.

## Performance and verification

The earlier refresh benchmark measured about **32â€“35% less time for navigation plus visible file/sidebar rendering** after avoiding redundant filesystem reads. That is not a claim about whole-app speed, terminal paint latency, or outperforming Everything. Forced reload costs were broadly unchanged in the measured run. The newer rendering pass reduces temporary allocations while ordinary navigation timing remains approximately unchanged; both comparisons are documented below.

- [Reproducible performance comparison](docs/benchmarks/refresh-optimization.md)
- [Rendering allocation and output-parity checks](docs/benchmarks/standout-render.md)
- [Verification history and limitations](docs/verification.md)
- [Remaining live acceptance checks](docs/completion-audit.md)
- [Configuration reference](website/src/content/docs/configure/superfile-config.mdx)

Local tests, lint, and cross-builds are not substitutes for live validation on every platform. Expanded features remain subject to the acceptance checks above.

The portability follow-up passed [Go CI on Windows, macOS and Linux](https://github.com/ASVLCII/Xelif/actions/runs/39833175305), the [Python integration suite](https://github.com/ASVLCII/Xelif/actions/runs/39833175330), and [module-tidy checks](https://github.com/ASVLCII/Xelif/actions/runs/39833175292). Those results apply to commit `6838f10`, not automatically to later changes; native interactive macOS/Linux acceptance and the Everything speed comparison remain separate gates.

## Attribution and license

Xelif is derived from **superfile**, created by [yorukot](https://github.com/yorukot) and its [contributors](https://github.com/yorukot/superfile/graphs/contributors). Upstream history and attribution are preserved.

[MIT License](LICENSE) Â· [Third-party notices](NOTICE.md) Â· [Upstream project](https://github.com/yorukot/superfile)

The source and upstream assets remain in the repository; this README's demo depicts Xelif rather than an inherited upstream screenshot.
