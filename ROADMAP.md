# Roadmap

## Milestone 1: Basic Ebuild Support (Current)
- [x] Basic parsing of variables (PN, PV, etc.)
- [x] SRC_URI parsing and URI extraction
- [x] Metadata XML parsing
- [x] Manifest file parsing and generation
- [x] Circular testing (Parse -> String -> Parse)

## Milestone 2: Enhanced Parsing & Writing
- [ ] Complete variable resolution (e.g., recursive expansion, shell substitutions)
- [ ] Parsing of ebuild functions (`src_compile`, `src_install`, etc.)
- [ ] Handling of conditional blocks (`if use ...`, `case ...`)
- [ ] Improved `inherit` handling and eclass awareness
- [ ] Support for all checksum algorithms in Manifests (currently limited set enabled by default)

## Milestone 3: Full Overlay Management
- [ ] Integration with `arrans_overlay_workflow_builder`
- [ ] Automated Manifest generation and signing
- [ ] Dependency graph resolution
- [ ] Overlay consistency checks (using `pkgcheck` logic internally)
- [ ] Full Git integration for overlay updates
