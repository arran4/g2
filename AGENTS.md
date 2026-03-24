# g2 Agents Instructions

* When verifying the static site generation output (e.g., using Playwright or headless Chrome), be aware that the `file://` protocol does not automatically resolve directory paths to `index.html`. Always start a local HTTP server (e.g., `python3 -m http.server -d <output_dir>`) before attempting to navigate through the generated site.
* For testing compile and run steps, use `go run ./cmd/g2 ...` rather than building a `g2` binary and executing it.
