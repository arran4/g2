# g2 Agents Instructions

## Engineering & Coding Standards

* **Deliberate Error Wrapping**: Wrap Go errors deliberately when it adds diagnostic context or preserves a caller-relevant error identity (e.g., `fmt.Errorf("reading category %s: %w", cat, err)` or `errors.Is`/`errors.As` sentinels), rather than mechanically wrapping at every stack frame or returning bare, uncontextualized errors.
* **Environment & Filesystem Boundaries**: Treat environment variables, process globals, and filesystem dependencies as application boundary concerns. For logic and unit tests, prefer explicit values, typed options, `io.Reader`/`io.Writer`, `fs.FS`, or `fstest.MapFS`. Reserve `t.TempDir()` and `t.Setenv()` for tests explicitly verifying OS/process boundary behavior (e.g., atomic file creation, permissions, or process environment lookups). Preserve real boundary integration coverage where required.
* **Test Fixtures & Anonymization**: Keep test fixtures small, named, deterministic, and readable. Place required static test data in `/testdata` or in-memory embedded fixtures (`fstest.MapFS`/`txtar`). All generated test data must be git-ignored. **Always anonymize individuals and strip real email addresses** in test fixtures (use generic names like "Jane Doe" and example domains like "example.com").
* **Repository Neutrality**: Do not hardcode specific remote URLs (e.g., `https://github.com/gentoo-mirror/...`) or Gentoo-specific links in generic templates or site generation features. `g2` supports multiple independent overlay repositories; ensure URLs and links are dynamic or configurable.
* **CLI & Documentation Ownership**:
  * `doc/g2.1.md` is the canonical maintained man-page source. Generated `md2man` output is build/test output and must not be committed to the repository.
  * Updates to man pages must follow conventional man-page standards (concise, declarative, using standard sections like NAME, SYNOPSIS, DESCRIPTION).
  * When adding or changing commands/features, agents must inspect the actual command tree using `go run ./cmd/g2 ...` and update both `doc/g2.1.md` and `readme.md`.
  * `readme.md` is the central user reference for available commands, flags, and usage examples. (Do not hand-edit generated README files if any exist.)
  * The `g2 dev` subcommand tree contains ephemeral local developer utilities (e.g. `g2 dev update-txtar-tests`) and does not need documentation in `readme.md`.
* **Testing & Verification Execution**:
  * For testing compile and run steps, use `go run ./cmd/g2 ...` rather than building a standalone `g2` binary.
  * When verifying static site generation output (e.g., Playwright/headless Chrome), note that `file://` does not automatically resolve directory paths to `index.html`. Use `go run ./cmd/g2 site serve -port 8080` or `python3 -m http.server` before navigation.

Further rationale: https://arran4.github.io/blog/post/2026/055-suggested-coding-standards-for-repositories-and-agents/
