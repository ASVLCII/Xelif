---
title: Xelif features
description: A concise guide to the file-management workflows available in the current Xelif development build.
head:
  - tag: title
    content: Features | Xelif
---

Xelif keeps the `spf` executable and superfile-compatible configuration while adding the workflows below. Click Commands or press `ctrl+shift+p` to open the command palette. New default bindings also include `alt+p`, which avoids Windows Terminal's own palette shortcut.

## Navigation and continuity

Xelif defaults to the customizable `monochrome` theme with neutral selection and focus indicators. Existing theme choices remain available through the `theme` configuration setting. There is no persistent file-action toolbar or back/forward/up button strip; breadcrumbs, context menus, and the searchable command palette keep those actions available.

### Mouse and keyboard

| Workflow | Mouse | Keyboard |
| --- | --- | --- |
| Open and navigate | Double-click an item; click a breadcrumb, sidebar folder, or tab | Enter, parent-directory and history bindings; panel/tab navigation bindings |
| Search and discover actions | Click Search or Commands (Find/Menu in narrow panels) | `ctrl+g` or `ctrl+shift+p` |
| Select several items | Ctrl-click toggles an item; Shift-click selects a range from the anchor | Selection mode, select/range/invert bindings |
| Browse results and commands | Mouse wheel; click a result or command; click outside to dismiss | Up/Down, Page Up/Page Down, Enter, Escape |
| Manage tabs and file actions | Click `[+]`/`[-]`; right-click an item; click a sort column or confirmation button | New/close tab bindings; `shift+f10` opens the same context menu; sort and action bindings |

Back/forward mouse buttons navigate the focused panel's history when the terminal reports them. Mouse modifiers also depend on terminal support; some terminals reserve Shift-click for text selection. Both input methods use the same file-operation and confirmation handlers.

New default bindings include `alt+p` for Commands and `alt+w` for closing a file tab, avoiding Windows Terminal's reserved Ctrl+Shift combinations. Existing custom bindings are preserved; the clickable Commands and `[-]` controls remain available.

### Compact file symbols

File entries stay on one row, with recognizable Nerd Font folder and file-type icons, matching the original superfile icon set. New profiles enable these icons by default; use a Nerd Font in your terminal. Only the icons receive category colors: blue folders/links, purple images, green documents, amber archives/disk images, cyan code/config, pink audio/video, and gray unknown files. Filenames and the monochrome interface remain neutral. Set `color_file_icons = false` for fully monochrome file lists, or `nerdfont = false` for basic Unicode fallback symbols. Plain mode remains text-only; `NO_COLOR` disables icon colors.

The palette selects light/dark variants for explicit theme backgrounds and falls back to the theme foreground if it cannot meet the contrast target. Transparent or unknown backgrounds use the theme foreground rather than assuming a readable category color. The repository's `docs/icon-colors.md` records the research and design rationale; no whole-application accessibility certification is implied.

Actual images are shown only in the optional large preview (`f` by default). Inline bitmap thumbnails and their loading/cache machinery have been removed.

- **Sessions and workspaces:** normal launches can restore panels, tabs, layout, sorting, and navigation history. In the Xelif prompt (`>`), use `workspace save <NAME>`, `workspace load <NAME>`, `workspace list`, or `workspace delete <NAME>` for named layouts.
- **Tabs and history:** each panel can hold multiple tabs with independent back and forward history. When tabs overflow, the active tab and create/close controls stay visible. Scroll over the tab strip to cycle all tabs; the next/previous-tab keyboard bindings reach the same tabs.
- **File watching:** open local directories refresh when filesystem changes are detected.

Set `restore_last_session = false` if every normal launch should start from the configured or current directory. Explicit command-line paths always take priority.

## Search, selection, and rename

- **Advanced search:** press `ctrl+g` to search from the focused directory. Queries support ordinary name terms plus `type:file`, `type:folder`, `ext:pdf`, `size:>2k`, `size:<3m`, `modified:<7d`, and quoted content filters such as `content:"net revenue"`.

On Windows, simple filename searches query the running Everything application directly through its native IPC interface; no SDK DLL or `es.exe` subprocess is needed for that path. The Everything service alone is insufficient: its user-session search process must also be running. If native IPC is unavailable, Xelif tries an installed `es.exe`, then searches beneath the focused folder with bounded work. Advanced filters currently use that local provider so their semantics remain consistent. No indexing service is started automatically by Xelif.

- **Saved searches:** use `search save <NAME> "<QUERY>"`, `search run <NAME>`, `search list`, and `search delete <NAME>` in the Xelif prompt. Saved searches also appear in the command palette.
- **Selection:** selection mode can select a range, invert the visible selection, or select only visible files or folders. The [hotkey list](/list/hotkey-list) shows the defaults.
- **Batch rename:** select the target items, then preview `batch-rename replace <FIND> <REPLACEMENT>`, `batch-rename prefix <TEXT>`, or `batch-rename suffix <TEXT>`. Nothing changes until `batch-rename apply`; use `batch-rename cancel` to discard the preview.

## Archives and project changes

- **Operation center:** choose **Operations** in Commands or click the Processes footer to inspect, filter and cancel tasks. Operations and Background tabs keep explicit file work separate from grouped automatic requests. Enter opens details; Ctrl+X requests cancellation, Ctrl+L clears finished history, and Escape returns without cancelling. The latest 100 finished records are kept for this session. Cancellation preserves completed changes and may wait for an active filesystem call. Services expose their status without a separate cancel control.

- **ZIP browsing:** choose **Browse ZIP as a folder** in the command palette to inspect a ZIP without unpacking it. Extraction rejects unsafe paths and preserves existing destination names instead of overwriting them.
- **Git:** file panels show repository status. The command palette can show working-tree or staged diffs, stage selected paths, unstage them, and discard tracked changes. Discard always requires confirmation.
- **Duplicates:** **Find duplicate files here** scans regular files under the focused directory and groups byte-identical files by SHA-256. The scan can be cancelled.
- **Directory compare:** with at least two panels open, compare the focused and next directories by metadata or content. Copy-missing synchronization is a one-way `focused → next` operation with separate preview and apply steps; existing destinations are not overwritten. Apply is bound to the reviewed files and their content. Changed inputs or panel paths require another preview; complete files already created during a later failure are retained and reported.

## Organization and metadata

- **Favourites:** right-click a folder and choose **Toggle favourite**, or press `shift+p` to pin the current folder. In the sidebar, `shift+p` acts on the focused entry; right-click or `shift+f10` opens its menu. Click a saved folder to navigate there. New default layouts put Favourites above Recent; custom section orders and existing `pinned.json` data remain compatible. Offline folders stay saved until explicitly removed. Unreadable or corrupt storage is preserved and writes fail visibly instead of resetting the list.
- **Tags:** use `tag add <TAG>`, `tag remove <TAG>`, `tag list`, and `tag find <TAG>`. Add and remove operate on the focused item or current selection.
- **Collections:** use `collection save <NAME>`, `collection open <NAME>`, `collection list`, and `collection delete <NAME>`. A collection saves the focused path or selection. Opening it shows a filterable result picker: use arrows and Enter, or click, to reveal any member in its local panel. Missing or filtered-out files show an inline error without losing the list. Tags use the same picker. Corrupt or unsupported catalog storage is preserved and writes are disabled rather than silently resetting it.
- **Metadata editing:** preview a modification time with `metadata modified <RFC3339>`, an access time with `metadata accessed <RFC3339>`, or permissions with `metadata permissions <OCTAL>`. Run `metadata apply` to commit the preview or `metadata restore` to restore the last applied edit from its journal.

Unix permission editing is available on Unix-like systems. Windows reports it as unsupported instead of pretending to apply POSIX mode bits; timestamp editing remains available.

## Terminal input and display

- **Inbound drag and drop:** when a supported terminal sends one or more local absolute paths, Xelif asks before copying them into the focused directory. Name collisions receive a safe suffix rather than overwriting an existing item.
- **Plain mode:** `plain_mode = true` uses ASCII borders, disables Nerd Font glyphs, and disables image previews. The standard `NO_COLOR` environment variable also disables color.
- **Language foundation:** `language = "auto"` selects from the environment; `en` and `zh-TW` are accepted explicit values. Translation coverage is still growing, so some interface text can remain English.

## Command plugins

Command plugins add explicit external-tool actions to the command palette. Put one version 1 JSON manifest per plugin in the `command-plugins` folder inside the [superfile-compatible config directory](/configure/config-file-path), then choose **Refresh command plugins** from the palette. Loaded actions appear as **Plugin: _Name_** and operate on the focused item or current selection.

```json
{
	"version": 1,
	"id": "inspect-json",
	"name": "Inspect JSON",
	"executable": "jq",
	"args": [".", "{path}"],
	"extensions": [".json"],
	"timeout_ms": 10000,
	"max_output_bytes": 1048576,
	"env_allowlist": []
}
```

`{path}` is the first selected path, `{paths}` expands to all selected paths when used as a complete argument, and `{dir}` is the first path's parent directory. Xelif invokes the executable directly—not through a shell—and rejects shell executables, parent traversal, unknown fields, credential-like environment variables, and invalid selections. Time and combined output are bounded by the manifest; captured output appears in Xelif's text viewer.

## Read-only remote files

Install [rclone](https://rclone.org/) and configure remotes with rclone itself. Choose **Browse rclone remotes** from the command palette to list configured remotes and navigate their folders. Press `enter` to open a remote or folder; on a file, `enter` or `d` downloads it into the focused local panel. The Open, Download, Back/Retry, and Close actions are also clickable.

The remote browser only lists and downloads: it does not upload, delete, rename, or otherwise mutate remote data. Downloads also refuse to overwrite an existing local filename. Interactive password prompts are disabled, so credentials must already be available to rclone.

## Compatibility

Xelif deliberately retains the `spf` executable, the `superfile` configuration and data directories, existing themes and hotkeys, and the upstream Go module path. This keeps existing superfile setups usable; see [config file paths](/configure/config-file-path) for the exact locations.
