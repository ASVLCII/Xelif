---
title: Plugin list
description: Built-in integrations and declarative command plugins available to Xelif.
head:
  - tag: title
    content: Plugin list | Xelif
---

Xelif retains superfile's optional integrations and adds declarative command plugins.

### Command plugins

- **Description:** Add external-tool actions to the command palette with versioned JSON manifests.
- **Requirements:** The executable named by each manifest must be installed.
- **Configuration:** Put manifests in the `command-plugins` folder inside the [config directory](/configure/config-file-path), then run **Refresh command plugins** from the command palette.
- **Safety:** Commands run directly without a shell, with an environment allowlist plus bounded execution time and output.

See the [feature guide](/features#command-plugins) for the schema and an example.

### Metadata

- **Description:** Show more detailed metadata for files and directories

- **Requirements:** [`exiftool`](https://exiftool.org)

- **Config name:** `metadata`

### MD5 Checksum

- **Description:** Show MD5 checksums for regular files in the metadata panel

- **Requirements:** None

- **Config name:** `enable_md5_checksum`

- **Note:** Calculating checksums reads the selected file, so it may be slow for large files.

### Zoxide

- **Description:** Smart directory jumping integration with zoxide. Navigate to frequently used directories quickly with a searchable modal interface.

- **Requirements:** [`zoxide`](https://github.com/ajeetdsouza/zoxide)

- **Config name:** `zoxide_support`

- **Usage:** Press `z` to open the zoxide navigation modal. Start typing to search directories, use arrow keys to navigate results, and press Enter to jump to a directory.
