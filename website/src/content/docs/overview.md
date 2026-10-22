---
title: Overview
description: What Xelif is, how it relates to superfile, and where to learn its current workflows.
head:
  - tag: title
    content: Overview | Xelif
---

![Demo of the terminal file manager interface inherited from superfile](../../assets/demo.png)

# What is Xelif?

Xelif is a keyboard-first terminal file manager built as a fork of [yorukot/superfile](https://github.com/yorukot/superfile). It retains superfile's polished multi-panel interface and adds searchable, safety-focused workflows for managing larger projects. Both projects are built with [Go](https://go.dev/) and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## What does Xelif add?

The current development build includes a command palette, sessions and named workspaces, richer selection and batch rename, advanced and saved searches, ZIP browsing, Git actions, tags and collections, duplicate detection, directory comparison and copy-missing sync, metadata editing, inbound terminal drag and drop, and an accessibility-oriented plain mode.

See the [Xelif feature guide](/features) for the exact user-facing behavior and commands.

## Compatibility and attribution

The executable remains `spf`, and configuration, data directories, themes, hotkeys, and the Go module path retain their superfile names for compatibility. Upstream artwork, package links, community links, and historical documentation remain attributed to [superfile and its contributors](https://github.com/yorukot/superfile). Xelif is distributed under the repository's [MIT License](https://github.com/yorukot/superfile/blob/main/LICENSE).
