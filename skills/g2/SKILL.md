# g2 Agent Skill

This skill provides operational guidance for AI coding agents to use the `g2` CLI successfully.

## Overview
`g2` is a multi-faceted command-line toolkit for Gentoo Linux overlays.
It generates static websites, checks repository lint rules, manages portage configuration, and manipulates packages and metadata.

## Important CLI Context

- **Main entrypoint:** Top-level commands are handled via a switch statement inside `cmd/g2/main.go`. When interacting with the tool, ensure you specify a valid subcommand.
- **TUI and Interactivity:** Some commands (like `g2 world`) launch an interactive Terminal User Interface (TUI). When running within a non-interactive shell (such as an automated agent session), avoid invoking TUI commands to prevent hanging processes.
- **Development environment:** To run the site server locally on standard environments, you will need to point to a valid overlay path, e.g., `go run ./cmd/g2 site serve .`
- **Output formatting:** A GitHub Actions output formatter is available for linting commands using `--format=github-actions`.
- **Parsing optimizations:** `g2` contains significant optimizations such as fast path resolution for Bash strings during variable resolution to speed up ebuild variable expansion.
- **Security notes:** Fetching network resources should carefully check schemes to prevent SSRF, and local modifications use robust write-and-rename semantics. The `SHA1` algorithm is explicitly deprecated.

## Common Subcommands
- `g2 site serve`: Start a local HTTP server for testing site generation templates.
- `g2 overlay site generate`: Render the repository pages to a static directory.
- `g2 lint`: Run QA policy and structural checks against the overlay.
- `g2 cache generate`: Create `md5-dict` (actually `sha256-dict`) cache files to speed up Portge syncs.
- `g2 package search`: Rapid package queries against the metadata index.
- `g2 skill`: Manage AI agent skills (install, update, list, remove).

## Best Practices for Agents
1. When debugging the `g2` internals, check the `MainArgConfig` struct to trace argument passing.
2. If modifying static site templates, note that the local dev server (`site_serve.go`) uses a `GenericPageContext`, which must be kept in sync with the generator context fields.
3. Be aware that the `repos.conf` parser differs from `make.conf` (which relies on `mvdan.cc/sh/v3/syntax`).
