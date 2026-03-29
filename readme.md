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

### `g2 cache`

commands relating to md5-dict/cache

#### `g2 cache clean`

To clean up unused cache entries

**Flags:**
```
-repo string
```

#### `g2 cache generate`

To generate cache for ebuilds

**Flags:**
```
-repo string
```

#### `g2 cache list-methods`

To list available cache methods

#### `g2 cache set-method`

To set the cache method in layout.conf

**Flags:**
```
-repo string
```

#### `g2 cache verify`

To verify cache exists for ebuilds

**Flags:**
```
-repo string
```

### `g2 ebuild`

commands relating to ebuild files

#### `g2 ebuild as-json`

Parse ebuild using native parser and output JSON

#### `g2 ebuild init`

Initialize an ebuild from a template

**Flags:**
```
-bdepend string
-depend string
-description string
-eapi string
-homepage string
-keywords string
-license string
-pep517 string
-python-compat string
-rdepend string
-slot string
-src-uri string
```

#### `g2 ebuild sh-parse-to-json`

Parse ebuild using shell parser and output JSON

#### `g2 ebuild templates`

Manage ebuild templates

##### `g2 ebuild templates init`

Initialize an ebuild from a specific template

**Flags:**
```
-bdepend string
-depend string
-description string
-eapi string
-homepage string
-keywords string
-license string
-pep517 string
-python-compat string
-rdepend string
-slot string
-src-uri string
```

##### `g2 ebuild templates list`

List available ebuild templates

### `g2 lint`

lints the repository for errors

### `g2 manifest`

commands relating to Manifest files

#### `g2 manifest clean`

To clean up the manifest from unused entries

#### `g2 manifest upsert-from-url`

To update or insert Manifest entries streamed from a URL

#### `g2 manifest verify`

To verify the manifest against ebuild files

**Flags:**
```
-clean
-fix
```

### `g2 metadata`

commands relating to metadata.xml files

**Flags:**
```
-force
-l string
-longdescription string
-m value
-maintainer value
-maintainer-add value
-maintainer-remove value
-u value
-upstream-add value
-upstream-id value
-upstream-remove value
-use value
-use-add value
-use-remove value
```

### `g2 overlay`

commands relating to a single overlay

### `g2 overlays`

commands relating to multiple overlays

### `g2 package`

commands relating to packages and search indexing

#### `g2 package index`

index local repositories

**Flags:**
```
-o string
-out-dir string
-out-zip string
-repo-filter string
-z string
```

#### `g2 package index-overlay`

index a single overlay

**Flags:**
```
-o string
-out-dir string
-out-zip string
-z string
```

#### `g2 package index-repositories`

index multiple repositories from an xml file

**Flags:**
```
-o string
-out-dir string
-out-zip string
-repo-filter string
-z string
```

#### `g2 package search`

search packages

**Flags:**
```
-path string
```

#### `g2 package update`

update the local index from a remote zip file

**Flags:**
```
-o string
-out-dir string
-url string
```

### `g2 pkg-desc-index`

commands relating to pkg_desc_index

#### `g2 pkg-desc-index generate`

Generate pkg_desc_index file from repository

**Flags:**
```
-repo string
```

#### `g2 pkg-desc-index verify`

Verify existing pkg_desc_index file matches repository

**Flags:**
```
-repo string
```

### `g2 site`

commands relating to static sites

### `g2 use`

commands relating to USE flags, use.desc, and use.local.desc

#### `g2 use desc-add`

Add a USE flag description to use.desc

**Flags:**
```
-desc string
-file string
-flag string
```

#### `g2 use desc-edit`

Edit a USE flag description in use.desc

**Flags:**
```
-desc string
-file string
-flag string
```

#### `g2 use desc-list`

List all USE flag descriptions from use.desc

**Flags:**
```
-file string
```

#### `g2 use desc-remove`

Remove a USE flag description from use.desc

**Flags:**
```
-file string
-flag string
```

#### `g2 use discover`

Discover USE flags from ebuilds and metadata.xml files to regenerate use.desc, use.local.desc, and metadata.xml.

**Flags:**
```
-file-desc string
-file-local-desc string
-metadata
-repo string
-use-desc
-use-local-desc
```

#### `g2 use local-desc-add`

Add a USE local flag description to use.local.desc

**Flags:**
```
-desc string
-file string
-flag string
-pkg string
```

#### `g2 use local-desc-edit`

Edit a USE local flag description in use.local.desc

**Flags:**
```
-desc string
-file string
-flag string
-pkg string
```

#### `g2 use local-desc-list`

List all USE local flag descriptions from use.local.desc

**Flags:**
```
-file string
```

#### `g2 use local-desc-remove`

Remove a USE local flag description from use.local.desc

**Flags:**
```
-file string
-flag string
-pkg string
```

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
