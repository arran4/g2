# g2 Agents Instructions

* When verifying the static site generation output (e.g., using Playwright or headless Chrome), be aware that the `file://` protocol does not automatically resolve directory paths to `index.html`. You can either start a local HTTP server (e.g. `python3 -m http.server -d <output_dir>`) or use the built-in testing server `go run ./cmd/g2 site serve -port 8080` before attempting to navigate through the generated site.
* Wrap returned errors only when a new layer contributes useful operation context or preserves a caller-relevant classification; avoid boilerplate wrapping in every helper. Use `%w` when errors.Is/errors.As identity is part of the contract, and do not log then return the same error repeatedly.
* Log actionable operation/path context at the command boundary and report intentional skips explicitly; avoid logging the same returned failure at every layer.
* For testing compile and run steps, use `go run ./cmd/g2 ...` rather than building a `g2` binary and executing it.
* Keep required on-disk test fixtures in `/testdata`; small named literals and in-memory fixtures may stay beside their tests. Keep generated test output in test-owned temporary storage or git-ignored paths, and do not commit it. Update golden fixtures only through an explicit update operation.
* Do not hardcode specific remote URLs (e.g. `https://github.com/gentoo-mirror/gentoo/blob/stable/` etc) in generic generation templates since the application is used against many different overlays. Ensure variables or configuration dictate correct URLs dynamically.
* Ensure generated site features remain repository neutral (e.g. do not hardcode links to Gentoo's GitHub or policy guide) as this tool supports importing multiple independent overlay repositories.

- `doc/g2.1.md` is the canonical maintained man-page source. The generated `md2man` output is build/test output and should not be committed to the repository.
- Man page updates should follow Unix manual-page conventions and be reference-oriented. Length should be driven by what users need to look up, not by an artificial brevity target: comprehensive, long manual pages are acceptable, as in mature Unix tools such as Perl and Zsh. Use conventional sections such as NAME, SYNOPSIS, DESCRIPTION, OPTIONS, FILES, ENVIRONMENT, EXAMPLES, EXIT STATUS, and SEE ALSO where applicable. Keep the prose precise and factual rather than marketing-oriented, and split material into additional manual pages only when that improves navigation, topical separation, or maintainability—not merely because the page is large.
- When adding or changing commands/features (like touching subcommands), agents must update:
  - `doc/g2.1.md` to reflect the new functionality.
  - `readme.md` where appropriate.
- Agents should inspect the real command tree using `go run ./cmd/g2 ...` to document commands accurately, rather than relying on stale markdown docs.

- When creating or modifying test data, **always anonymize individuals and strip real email addresses**. Use generic names (e.g. "Jane Doe") and example domains (e.g. "example.com").

- When adding new commands or features to the `g2` application, ensure that you update the `readme.md` file to reflect these additions. The `readme.md` should be considered the central reference for available commands, usage instructions, and examples.

- There is a `g2 dev` subcommand tree specifically for ephemeral tools used by developers and agents (e.g. `g2 dev update-txtar-tests`). These commands do not need to be documented in `readme.md` and are for local utility purposes.

## Adding or Changing Commands
When adding or changing g2 commands, agents must inspect the actual command tree via the CLI (e.g., `go run ./cmd/g2 ...`) instead of relying on stale docs, and update both `readme.md` and the canonical man page source `doc/g2.1.md` using conventional man-page sections.

## Side effects and test fixtures

- A dry-run or verification path without explicit write flags must not change a destination overlay, tracked templates or user state. Render candidate changes separately and apply only in the explicit write mode (for example, manifest verification with `--fix` or `--clean`); use temporary storage for any necessary file comparison.
- Resolve environment variables, current working directory and process streams at application boundaries. Prefer explicit paths/options, `io.Reader`/`io.Writer` and the existing `fs.FS` seams for inner logic; do not introduce broad dependency wrappers or silently change public APIs.
- Keep parser fixtures small, named, deterministic and readable; use `fstest.MapFS` or readable txtar fixtures for file trees when the OS is not under test. Keep focused real Gentoo/Portage/filesystem integration tests for behaviour that depends on those tools, permissions, atomic writes or on-disk layout. Use `t.TempDir()` for those OS contracts and `t.Setenv()` for actual environment wiring, keeping process-global mutations out of parallel tests. Existing issue #572 owns broader fixture and process-state cleanup.
- Treat committed/generated output as a reproducible artifact. Verify changes against the source template and inspect exact output before applying or committing; do not hand-edit generated README files.

## Engineering guidance and agent audit

Use https://arran4.github.io/blog/post/2026/055-suggested-coding-standards-for-repositories-and-agents/ selectively, without superseding the more specific rules above. Environment/test-boundary rationale: https://arran4.github.io/blog/post/2026/054-stop-rebuilding-xdg-directories-in-go-tests/. A coding agent must inspect implementation/tests, distinguish verified gaps from hypotheses, and record results and existing issue overlap at https://github.com/arran4/g2/issues/577. Do not equate adding instructions with completing the implementation audit; do not merge automatically.
