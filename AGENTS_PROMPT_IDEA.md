# Prompt Idea for Advanced AST-Aware Configuration Editor

To build the "higher tier" editor for Gentoo configuration files (`make.conf`, `repos.conf`, `layout.conf`), you should break down the implementation into several distinct PRs, as the scope is massive.

## Phase 1: AST Parsing and Provenance Tracking
**Goal**: Build the core data models and utilities that read configuration files while preserving their exact abstract syntax tree, formatting, and file origins (including `.d` directories).

**Prompt to provide to Agent:**
"Please implement an AST-aware configuration parser for Portage's `make.conf` and `repos.conf`. We cannot use a simple line-by-line approach because we must preserve comments, exact indentation, and formatting when saving. Furthermore, we must track the provenance of every variable or block (e.g., if `make.conf` sources files from `make.conf.d/`, or if `repos.conf` is a directory, the parsed AST must record exactly which file and which line a value originated from).

Use `mvdan.cc/sh/v3/syntax` to parse `make.conf` files into an AST. Implement a `ConfigurationModel` struct that holds the root AST nodes and can map variables (like `COMMON_FLAGS`) back to their specific `*syntax.Assign` nodes and their origin file paths. Do not implement a UI yet, focus purely on the AST models, parsing logic (handling directories), and the logic required to accurately write an altered AST back to its original file."

## Phase 2: Core TUI Framework and Extensible Widgets
**Goal**: Build a flexible TUI framework (likely using a library like `bubbletea` or `tview` for complex layouts) that can display groups of settings rather than raw text lines, and supports plugging in custom editors.

**Prompt to provide to Agent:**
"Building on the AST parsers from Phase 1, we need to create a new terminal UI framework in `cmd/g2` that moves away from raw text editing. The UI should display configuration settings as logical 'groups' (e.g., a section for USE flags, a section for CFLAGS).

Please implement the base UI framework. It must support displaying the origin file for each setting (derived from Phase 1). Crucially, the framework must be designed to accept 'Custom Editor Widgets'. As a proof-of-concept, implement a simple text-field widget for generic variables, and a specialized 'Checkbox List Widget' that we will later use for USE flags. The UI must be able to read from the AST model and, upon saving, update the specific AST nodes and trigger the write-back logic."

## Phase 3: Specialized Widgets and Semantics
**Goal**: Implement the specialized editors requested (USE flags, CFLAGS/CPU-ID, LC_MESSAGES, Licenses) and the complex merge semantics.

**Prompt to provide to Agent:**
"Now we will implement specialized editors for the AST-aware TUI.
1.  **USE Flags & Licenses**: Implement a checkbox-based editor widget for `USE` and `ACCEPT_LICENSE` variables. It should parse the string array from the AST, present them as toggles, and allow adding new ones.
2.  **LC_MESSAGES**: Implement a single-selection dropdown/list widget for `LC_MESSAGES` that offers the available standard options.
3.  **CFLAGS / COMMON_FLAGS**: Implement an advanced editor for `COMMON_FLAGS`. It must feature a special 'mode' or button to invoke a local CPU-ID tool to auto-populate optimal `-march` and `--param` flags.
4.  **Append Semantics**: Add support in the UI for variables that are appended to others (e.g., `CFLAGS="${COMMON_FLAGS}"`). The editor must allow the user to easily specify whether new flags (like an apache module) should be injected into the base `COMMON_FLAGS` or directly appended to the end of `CFLAGS`.

Ensure that modifying any of these widgets correctly updates the underlying AST nodes created in Phase 1, preserving all comments and indentation in the final files."
