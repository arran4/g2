# Roadmap

## Milestone 1: Basic Ebuild Support (Current)
- [x] Basic parsing of variables (PN, PV, etc.)
- [x] SRC_URI parsing and URI extraction
- [x] Metadata XML parsing
- [x] Manifest file parsing and generation
- [x] Circular testing (Parse -> String -> Parse)

## Milestone 2: Enhanced Parsing & Writing
- [ ] Complete variable resolution (e.g., recursive expansion, shell substitutions)
- [x] Parsing of ebuild functions (`src_compile`, `src_install`, etc.)
- [ ] Handling of conditional blocks (`if use ...`, `case ...`)
- [x] Improved `inherit` handling and eclass awareness
- [x] Support for all checksum algorithms in Manifests (currently limited set enabled by default)

## Milestone 3: Full Overlay Management
- [x] Automated Manifest generation
- [x] Dependency graph resolution
- [x] Overlay consistency checks (using `pkgcheck` logic internally)
- [x] Full Git integration for overlay updates

## Milestone 4: Advanced Web Dashboard & Automation
- [ ] PGP signing support for Manifests
- [ ] Automated package version bumping and PR generation
- [ ] Web-based visualization of dependency graphs
- [ ] Expose QA and linting reports directly in the generated web dashboard
