# Xelif preview

A mouse- and keyboard-accessible terminal file manager based on
[superfile](https://github.com/yorukot/superfile), developed by
[ASVLCII](https://github.com/ASVLCII/Xelif).

These commands become available **after the first npm publication**:

```sh
npm install -g @asvlcii/xelif@preview
# or
bun add -g @asvlcii/xelif@preview

xelif
xelif "/path/to/a folder"
```

The `preview` tag is intentional; this is not a stable release. A Node.js 22+
runtime is required by the global launcher. The launcher is also compatible with
Bun's runtime when invoked with Bun explicitly.

The same public npm package works with npm and Bun. An OS/CPU-specific optional
dependency contains the real Go executable. Do not omit optional dependencies.
Windows, macOS and Linux have x64/arm64 build targets. Their CI installation
checks are distinct from interactive terminal testing on those machines.

No postinstall scripts, runtime downloads, SDK DLLs, or Go compiler are needed
to run the installed package. Configuration, hotkeys and themes are embedded in
the executable. Native packages include the upstream license and notices.

Use a Nerd Font for the illustrated file-type icons. Optional tools such as
Everything (Windows indexed search), ffmpeg and exiftool are not bundled.
Everything's desktop application must be running; its service alone is not
enough. Local search remains available, but Everything-level speed has not been
established by the current comparison.

Configuration keeps the existing superfile-compatible locations. Run
`xelif path-list` to inspect them. A packaged install does not hardcode a drive,
open another terminal, install fonts, or overwrite your terminal settings.

On the development PC, the existing machine-local `Xelif.cmd` may precede the npm
shim on PATH. This package does not overwrite that launcher. Inspect `Get-Command
xelif -All` (PowerShell) or `command -v xelif` (Unix) if a different installation
opens. Preview executables are not Windows Authenticode signed or Apple-notarized.

Source, issues and verification: https://github.com/ASVLCII/Xelif
