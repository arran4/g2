# g2 Agents Instructions

* When verifying the static site generation output (e.g., using Playwright or headless Chrome), be aware that the `file://` protocol does not automatically resolve directory paths to `index.html`. You can either start a local HTTP server (e.g., `python3 -m http.server -d <output_dir>`) or use the built-in testing server `go run ./cmd/g2 site serve -port 8080` before attempting to navigate through the generated site.
* Ensure most return err are wrapped with a meaningful addition to where the error is occurring and/or the context it's occurring in.
* Increase logging output.
* For testing compile and run steps, use `go run ./cmd/g2 ...` rather than building a `g2` binary and executing it.
* All test data should be in `/testdata` if it is required for tests. Otherwise, test generated data must be git ignored and not added to the repo.
* Do not hardcode specific remote URLs (e.g. `https://github.com/gentoo-mirror/gentoo/blob/stable/` etc) in generic generation templates since the application is used against many different overlays. Ensure variables or configuration dictate correct URLs dynamically.
* Ensure generated site features remain repository neutral (e.g. do not hardcode links to Gentoo's GitHub or policy guide) as this tool supports importing multiple independent overlay repositories.

- When creating or modifying test data, **always anonymize individuals and strip real email addresses**. Use generic names (e.g., "Jane Doe") and example domains (e.g., "example.com").

- When adding new commands or features to the `g2` application, ensure that you update the `readme.md` file to reflect these additions. The `readme.md` should be considered the central reference for available commands, usage instructions, and examples.
