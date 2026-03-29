# g2

Some Gentoo CLI tools I wrote for myself.

*   Repository: [https://github.com/arran4/g2](https://github.com/arran4/g2)
*   Go package docs: [https://pkg.go.dev/github.com/arran4/g2](https://pkg.go.dev/github.com/arran4/g2)

## Installation

You can install `g2` using Go:

```bash
go install github.com/arran4/g2/cmd/g2@latest
```

Alternatively, clone the repository and build manually:

```bash
git clone https://github.com/arran4/g2.git
cd g2
go build -o g2 ./cmd/g2
```

## Quick Start

```bash
# Generate a manifest for a downloaded package
g2 manifest upsert-from-url https://example.com/software-1.0.tar.gz software-1.0.tar.gz .

# Add a maintainer to an ebuild's metadata.xml
g2 metadata -maintainer-add "dev@example.com:Developer Name:person" .

# Generate an HTML site for your overlay
g2 overlay site generate -out site_out .

# Lint your overlay for errors
g2 lint /var/db/repos/my-overlay
```

## Command Reference

{{.CommandReference}}

## Modules / Functional Groupings

The commands naturally group into several functional areas depending on what Gentoo component they affect:

*   **Manifests**: `g2 manifest` handles checksum tracking and distfile verification.
*   **Metadata**: `g2 metadata` assists in managing package owners, descriptions, and remote IDs in `metadata.xml`.
*   **Ebuilds**: `g2 ebuild` provides templating and parsing for ebuild files.
*   **Site Generation**: `g2 overlay site` and `g2 overlays site` generate static web dashboards for Gentoo repositories.
*   **USE Flags**: `g2 use` manages and discovers use flags across the repository.
*   **Cache**: `g2 cache` supports local repository cache generation.
*   **Search**: `g2 package` supports searching locally and updating remote indexes.

## GitHub Action

You can use the [g2 GitHub Action](https://github.com/arran4/g2-action) to integrate `g2` into your CI/CD workflows. It automatically downloads and installs `g2` for use in your workflow steps.

### Example Usage

```yaml
name: Example workflow
on: [push]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install g2
        uses: arran4/g2-action@v1
        with:
          # Optional: specify a version. Defaults to 'latest'
          version: 'latest'

      - name: Use g2
        run: |
          g2 lint .

      - name: Use g2 Action
        uses: arran4/g2-action@v1.2
        with:
          # Optional: specify an action
          action: 'lint .'
          # Optional: skip installation if already installed
          mode: 'run'
```

### Search Features
`g2 site` features a fully integrated browser-side search accessible from the main dashboard.
Search indexes are emitted automatically to `search/data`.
Users can query package names, descriptions, use flags, and numerous field filters (`category`, `license`, `mask`, `version`, `depends`, etc.).
Advanced queries support boolean logic (AND, OR, NOT), grouping `()`, and sequence matching (`'sequence of words'`).
Gentoo version ordering is natively supported for range queries like `version:>1.2.3`.

---
*Note: The command reference is generated from the CLI definition. Use `make readme` to update the command reference.*
