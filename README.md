# Xelif

Xelif is a mouse- and keyboard-accessible terminal file manager built as a fork of [yorukot/superfile](https://github.com/yorukot/superfile). It keeps superfile's multi-panel workflow and adds searchable tools for larger file-management jobs.

## Development status

This repository is a development snapshot, not a stable Xelif release. Local tests,
builds, performance results, and remaining live acceptance checks are documented in
[the completion audit](docs/completion-audit.md) and
[the latest performance report](docs/benchmarks/refresh-optimization.md).
The inherited upstream screenshots and download links below are not Xelif releases.

To build this checkout with the Go version specified in `go.mod`:

```sh
go build -o bin/xelif .
```

On Windows, use `go build -o bin/xelif.exe .` instead. The launcher setup described
in `docs/local-launch.md` is specific to the original development machine.

> [!IMPORTANT]
> Xelif is still compatible with superfile's `spf` command, configuration directories, themes, hotkeys, and Go module path. Those technical names are intentionally unchanged while the fork evolves.

## Xelif features

- Start with a customizable monochrome theme. Click breadcrumbs, tabs, sidebar folders, sort columns, Search, and Commands, or use their keyboard equivalents. There is no persistent bottom action toolbar.
- Search filenames through Everything's native Windows interface when its user-session application is running, with optional CLI and bounded current-folder fallbacks. Advanced filters use the local provider; no speed advantage over Everything is claimed.
- Search and run actions from the command palette; search across files with type, extension, size, modified-time, and text-content filters, then save useful queries.
- Restore the last session, save named workspaces, and use independent tabs and navigation history inside panels. Watched folders refresh when their contents change.
- Select ranges, invert selections, or target visible files and folders; preview batch renames before applying them.
- Browse ZIP archives without extracting them first, and extract without overwriting existing files.
- See Git status in file panels, inspect working-tree or staged diffs, stage, unstage, and discard tracked changes only after confirmation.
- Organize paths with tags and collections, find duplicate files, compare directories, and preview copy-missing synchronization before applying it.
- Preview timestamp or permission changes, apply them explicitly, and restore the last metadata edit from its journal.
- Use plain mode for an ASCII, no-color-friendly display and choose the initial UI language (`auto`, English, or Traditional Chinese).
- Drop local paths into a supported terminal and confirm before Xelif copies them into the focused directory.
- Add declarative command plugins to the command palette; Xelif invokes their executables directly with bounded time and output.
- Browse configured rclone remotes without changing them, and download individual files without overwriting local files.

See the [Xelif feature guide](./website/src/content/docs/features.md) for the exact commands, filters, and safety boundaries.

## Upstream and license

Xelif is derived from [superfile](https://github.com/yorukot/superfile), created by [yorukot](https://github.com/yorukot) and its contributors. The inherited `superfile` name, `spf` command, configuration paths, artwork, package links, and community links below refer to that upstream project. Xelif remains available under the repository's [MIT License](./LICENSE), with third-party notices in [NOTICE.md](./NOTICE.md).

<div align="center">

<h4>superfile is supported by the community.</h4>

<a href="https://ko-fi.com/yorukot">
  <img alt="Donate to superfile on Ko-fi" src="https://ko-fi.com/img/githubbutton_sm.svg">
</a>

<hr>

</div>

<div align="center">
<br>
<picture>
  <source width="300" media="(prefers-color-scheme: dark)" srcset="website/src/assets/superfile-night.svg" />
  <source width="300" media="(prefers-color-scheme: light)" srcset="website/src/assets/superfile-day.svg" />
  <img alt="superfile LOGO" src="website/src/assets/superfile-day.svg" />
</picture>
<br><br>

[![License MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/yorukot/superfile/refs/heads/main/LICENSE) [![Discord Link](https://img.shields.io/discord/1338415256875307110?label=discord&logo=discord&logoColor=white)](https://discord.gg/YYtJ23Du7B) [![Release](https://img.shields.io/github/v/release/yorukot/superfile.svg?style=flat-square)](https://github.com/yorukot/superfile/releases/latest) ![Homebrew downloads](https://img.shields.io/homebrew/installs/dy/superfile?label=Homebrew) ![GitHub downloads](https://img.shields.io/github/downloads/yorukot/superfile/total?label=GitHub%20downloads%20assets%2Freleases) [![CodeRabbit Pull Request Reviews](https://img.shields.io/coderabbit/prs/github/yorukot/superfile?utm_source=oss&utm_medium=github&utm_campaign=yorukot%2Fsuperfile&labelColor=171717&color=FF570A&&label=CodeRabbit+Reviews)](https://www.coderabbit.ai/)

![](website/src/assets/demo.png)

</div>

## Demo

| Perform common operations  |
| -------------------------- |
| ![](asset/readme/demo.gif) |

## Content

- [Xelif features](#xelif-features)
- [Upstream and license](#upstream-and-license)
- [Demo](#demo)
- [Content](#content)
- [Installation](#installation)
  - [macOS and Linux](#macos-and-linux)
  - [Windows](#windows)
    - [Powershell](#powershell)
    - [Winget](#winget)
    - [Scoop](#scoop)
  - [More installation methods](#more-installation-methods)
- [Build](#build)
  - [For macOS/Linux](#for-macoslinux)
  - [For Windows](#for-windows)
- [Start superfile](#start-superfile)
- [Supported Systems](#supported-systems)
- [Tutorial](#tutorial)
- [Plugins](#plugins)
- [Themes](#themes)
- [Hotkeys](#hotkeys)
- [Notes](#notes)
- [Troubleshooting](#troubleshooting)
- [Uninstalling](#uninstalling)
  - [macOS and Linux](#macos-and-linux-1)
  - [Windows](#windows)
- [Contributing](#contributing)
- [Thanks](#thanks)
  - [Support](#support)
  - [Core maintainer](#core-maintainer)
  - [Contributors](#contributors)
  - [Powered by](#powered-by)
  - [Star History](#star-history)
- [༼ つ ◕\_◕ ༽つ Please share.](#-つ-_-つ-please-share)

## Installation

> [!CAUTION]
> Xelif does not yet have a separate packaged release. The scripts and package-manager commands below install upstream superfile. To run the Xelif features in this checkout, use the **Build** section.

### macOS and Linux

```bash
bash -c "$(curl -sLo- https://superfile.dev/install.sh)"
```

If you want to inspect the script, see : [install.sh](./website/public/install.sh)

### Windows

#### Powershell

```powershell
powershell -ExecutionPolicy Bypass -Command "Invoke-Expression ((New-Object System.Net.WebClient).DownloadString('https://superfile.dev/install.ps1'))"
```

If you want to inspect the script, see : [install.ps1](./website/public/install.ps1)

#### [Winget](https://winget.run/)

```powershell
winget install --id yorukot.superfile
```

#### [Scoop](https://scoop.sh/)

```
scoop install superfile
```

### More installation methods

[Click me to check on how to install](https://superfile.dev/getting-started/installation/)

## Build

You can build the source code yourself by using these steps:

**Requirements**

- [golang](https://go.dev/doc/install)

**Build Steps**

Run these commands from the root of this Xelif checkout. The output binary remains named `spf` for compatibility.

### For macOS/Linux

Run the `build.sh` file:

```bash
./build.sh
```

Add the binary file to your $PATH, e.g., in `/usr/local/bin`:

```bash
sudo mv ./bin/spf /usr/local/bin
```

### For Windows

```bash
go build -o bin/spf.exe
```

Edit System Environment Variables and add superfile repo's `bin` directory to your PATH

## Start superfile

```bash
spf
```

## Supported Systems

- \[x\] Linux
- \[x\] macOS
- \[x\] Windows (Not fully supported yet)

## Tutorial

After you install superfile, you can go [here](https://superfile.dev/getting-started/tutorial/) to briefly understand how to use superfile!

## Plugins

[Click me to the plugins wiki](https://superfile.dev/list/plugin-list/)

## Themes

[Click me to the theme wiki](https://superfile.dev/configure/custom-theme/)

## Hotkeys

> [!WARNING] If you are vim/nvim user please change your default hotkeys config to vim version!

[**Click me to see the hotkey wiki**](https://superfile.dev/configure/custom-hotkeys/)

## Notes

We have an auto update functionality, that fetches superfile's latest released version from github (if last timestamp of last version check was less than 24 hours) and prints a prompt to user, if there is a newer version available.

You can turn this off, by setting `auto_check_update` to false in superfile config. [**Click me to see the config wiki**](https://superfile.dev/configure/superfile-config/)

## Troubleshooting

[**Click me to see common problem fix**](https://superfile.dev/troubleshooting/)

## Uninstalling

### macOS and Linux

```bash
bash -c "$(curl -sLo- https://superfile.dev/uninstall.sh)"
```

If you want to inspect the script, see : [uninstall.sh](./website/public/uninstall.sh)

### Windows

To uninstall superfile on Windows, use this powershell script.

```powershell
powershell -ExecutionPolicy Bypass -Command "Invoke-Expression ((New-Object System.Net.WebClient).DownloadString('https://superfile.dev/uninstall.ps1'))"
```

## Contributing

If you want to contribute please follow the [contribution guide](./CONTRIBUTING.md)

[**Click me to see changelog**](https://superfile.dev/changelog)

## Thanks

### Support

- a Star on my GitHub repository would be nice 🌟
- You can buy a coffee for me 💖

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/G2G1JEGGC)

### Core maintainer

> We welcome anyone who wants to become a core maintainer. Feel free to reach out!

- **[@yorukot](https://github.com/yorukot)** - Original author and maintainer
- **[@lazysegtree](https://github.com/lazysegtree)** - Core maintainer

### Contributors

**Thanks to all the contributors for making this project even greater!**

<a href="https://github.com/yorukot/superfile/graphs/contributors">
  <img src="https://gthanks.yorukot.me/image?target=yorukot%2Fsuperfile" />
</a>

### Powered by

<a href="https://jb.gg/OpenSource"><img alt="JetBrains logo" align="right" width="200" src="https://resources.jetbrains.com/storage/products/company/brand/logos/jetbrains.svg"></a>

Thanks to JetBrains team for providing open-source licenses to support the maintenance of superfile.

### Star History

**THANKS FOR All OF YOUR STARS!** Your stars are my motivation to keep updating!

<a href="https://www.star-history.com/?repos=yorukot%2Fsuperfile&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=yorukot/superfile&type=date&theme=dark&legend=top-left&sealed_token=nEoMOYsfp1zwZ7rT-Fm6VR2yTa6cwW35VR0BwVxTuE8Dt17vRcRIQUFXeWdh6lZixlAl5e_fIVFs2Xe4cRdvAnexR5Q6JqlGVZK05Iu0mko8gYLjTdjq0g" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=yorukot/superfile&type=date&legend=top-left&sealed_token=nEoMOYsfp1zwZ7rT-Fm6VR2yTa6cwW35VR0BwVxTuE8Dt17vRcRIQUFXeWdh6lZixlAl5e_fIVFs2Xe4cRdvAnexR5Q6JqlGVZK05Iu0mko8gYLjTdjq0g" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=yorukot/superfile&type=date&legend=top-left&sealed_token=nEoMOYsfp1zwZ7rT-Fm6VR2yTa6cwW35VR0BwVxTuE8Dt17vRcRIQUFXeWdh6lZixlAl5e_fIVFs2Xe4cRdvAnexR5Q6JqlGVZK05Iu0mko8gYLjTdjq0g" />
 </picture>
</a>

<div align="center">

## ༼ つ ◕_◕ ༽つ Please share.

</div>
