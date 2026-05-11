# `pulumi neo` (`pkg/cmd/pulumi/neo/`)

The `pulumi neo` TUI/agent command.

## Changelog

- Use the `cli/neo` scope for changelog entries that touch this directory.

## Scrollback emission

- Use `m.printlnBlock(rendered)` to commit a block to scrollback. Do not call `tea.Println` directly for block content.
- `printlnBlock` enforces the blank-line gap above each block (and skips it for the very first emission), so spacing stays uniform across welcome banners, conversation turns, tool output, and other block kinds.
- `tea.Println` is fine for one-off raw lines that are not blocks (e.g. low-level debug output), but anything rendered as a TUI block must go through `printlnBlock`.
