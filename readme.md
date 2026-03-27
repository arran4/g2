# g2

Some Gentoo CLI tools I wrote for myself.

## Manifest Tools

`g2` includes a set of tools for working with Gentoo `Manifest` files.

### `manifest`

The `manifest` command group handles operations related to Manifest files.

#### `upsert-from-url`

Updates or inserts a `Manifest` entry for a file downloaded from a URL. This command streams the file, calculates the specified checksums, and updates the `Manifest` file in the specified directory (or specific file path).

**Usage:**

```bash
g2 manifest [flags] upsert-from-url <url> <filename> <manifestFileOrDir>
```

**Arguments:**

*   `<url>`: The HTTP/HTTPS URL of the file to download.
*   `<filename>`: The filename to record in the `Manifest` (typically the distfile name).
*   `<manifestFileOrDir>`: The path to the `Manifest` file or the directory containing it (e.g., the ebuild directory).

**Flags:**

The following flags control which checksums are calculated. Note that `BLAKE2B` and `SHA512` are enabled by default.

*   `-blake2b` (default: `true`): Calculate BLAKE2B checksum.
*   `-blake2s` (default: `false`): Calculate BLAKE2S checksum.
*   `-md5` (default: `false`): Calculate MD5 checksum.
*   `-rmd160` (default: `false`): Calculate RMD160 checksum.
*   `-sha1` (default: `false`): Calculate SHA1 checksum.
*   `-sha256` (default: `false`): Calculate SHA256 checksum.
*   `-sha3_256` (default: `false`): Calculate SHA3-256 checksum.
*   `-sha3_512` (default: `false`): Calculate SHA3-512 checksum.
*   `-sha512` (default: `true`): Calculate SHA512 checksum.

**Example:**

To download a package, calculate SHA256 in addition to defaults, and update the Manifest in the current directory:

```bash
g2 manifest -sha256=true upsert-from-url https://example.com/software-1.0.tar.gz software-1.0.tar.gz .
```

#### `verify`

Verifies the `Manifest` against the actual ebuild files.

**Usage:**

```bash
g2 manifest verify [location]
```

#### `clean`

Cleans up unused entries from the `Manifest` file.

**Usage:**

```bash
g2 manifest clean [location]
```

## Metadata Tools

`g2` includes tools to manage `metadata.xml` files.

### `metadata`

Commands relating to modifying `metadata.xml` files.

**Usage:**

```bash
g2 metadata [flags] [location]
```

**Flags:**

*   `-force`: Force overwrite if type mismatches or other errors
*   `-l, -longdescription <string>`: Set long description
*   `-m, -maintainer, -maintainer-add <email[:name[:type]]>`: Add/Update maintainer
*   `-maintainer-remove <email>`: Remove maintainer
*   `-u, -upstream-id, -upstream-add <type:id>`: Add upstream remote ID
*   `-upstream-remove <type:id>`: Remove upstream remote ID
*   `-use, -use-add <name:description>`: Add/Update USE flag
*   `-use-remove <name>`: Remove USE flag

**Example:**

Add a maintainer to `metadata.xml`:

```bash
g2 metadata -maintainer-add "dev@example.com:Developer Name:person" .
```

Add a USE flag description:

```bash
g2 metadata -use-add "custom-flag:Enables a custom feature" .
```

## Ebuild Tools

Tools for working with `.ebuild` files.

### `ebuild`

**Subcommands:**

*   `init <template_name>`: Initialize an ebuild from a template.
*   `templates`: Manage ebuild templates.
*   `sh-parse-to-json <ebuild_file>`: Parse an ebuild using the shell parser and output JSON.
*   `as-json <ebuild_file>`: Parse an ebuild using the native parser and output JSON.

**Usage Examples:**

Parse an ebuild natively into JSON format:

```bash
g2 ebuild as-json my-package-1.0.ebuild
```

Initialize a new ebuild from the generic template:

```bash
g2 ebuild init generic my-new-package-1.0.ebuild
```

## Site Generation

`g2` can generate a static HTML website representing your Gentoo overlay, including categories, packages, metadata, and git commit history. It also supports aggregating multiple remote overlays.

### `overlay site generate`

Generates a static site for a single overlay.

**Usage:**

```bash
g2 overlay site generate [-out <dir>] [-clear] [<location>]
```

**Arguments:**

*   `<location>`: Path to the overlay directory, or a Git URL (defaults to `.`).

**Flags:**

*   `-out`: Output directory for the generated site (default: `site_out`).
*   `-clear`: Clear output directory before generation.
*   `-fast-git-modtime`: Use fast (O(1)) but potentially less reliable go-git file log lookup.
*   `-recent-duration`: Duration to consider an update 'recent' (e.g., 3mo, 14d, 72h) (default "3mo").

**Example:**

```bash
g2 overlay site generate -out my_site_dir .
```

### `overlay ebuild`

Tools to modify the overlay with ebuilds.

**Subcommands:**
*   `move <from> <to>`: Record a package move in profiles/updates.
*   `slotmove <package> <from> <to>`: Record a slot move in profiles/updates.
*   `install [-category <string>] <ebuild.ebuild> [overlay_path] [-- <files...>]`: Install an ebuild into the overlay, optionally providing a specific category or additional files for the `files/` directory. Automatically triggers manifest, cache, use desc, and pkg_desc_index generation.

**Example:**

Install a package into a specific category with some files:
```bash
g2 overlay ebuild install -category sys-apps my-app-1.0.ebuild . -- app.conf app.service
```

### `overlays site generate`

Generates an aggregated static site for multiple remote repositories from a `repositories.xml` file.

**Usage:**

```bash
g2 overlays site generate [-out <dir>] [-clear] <repositoriesFile>
```

**Arguments:**

*   `<repositoriesFile>`: Path or URL to a Gentoo `repositories.xml` file, or `-` for stdin.

### `site serve`

Serves the generated static site locally for previewing.

**Usage:**

```bash
g2 site serve [-port <int>] [path_to_overlay]
```

**Example:**

```bash
g2 site serve -port 8080 .
```

## Linting

`g2` provides linting to ensure overlay consistency.

### `lint`

Checks the repository for errors such as ebuild `IUSE` variables missing in `metadata.xml` and missing `md5-cache` files.

**Usage:**

```bash
g2 lint [<location>]
```

**Arguments:**

*   `<location>`: Path to the overlay directory (defaults to `.`).

**Example:**

```bash
g2 lint /var/db/repos/my-overlay
```

## USE Flag Tools

Manage and discover USE flags, `use.desc`, and `use.local.desc`.

### `use`

**Subcommands:**

*   `discover [location]`: Discover USE flags from ebuilds and `metadata.xml` to regenerate `use.desc`, `use.local.desc`, and `metadata.xml`.
*   `desc-add <name> <description>`: Add a USE flag description to `use.desc`.
*   `desc-remove <name>`: Remove a USE flag description from `use.desc`.
*   `desc-edit <name> <description>`: Edit a USE flag description in `use.desc`.
*   `desc-list`: List all USE flag descriptions from `use.desc`.
*   `local-desc-add <pkg> <name> <description>`: Add a USE local flag description to `use.local.desc`.
*   `local-desc-remove <pkg> <name>`: Remove a USE local flag description from `use.local.desc`.
*   `local-desc-edit <pkg> <name> <description>`: Edit a USE local flag description in `use.local.desc`.
*   `local-desc-list`: List all USE local flag descriptions from `use.local.desc`.

**Example Usage:**

Discover and populate USE flags automatically:

```bash
g2 use discover .
```

List all local USE descriptions:

```bash
g2 use local-desc-list
```

## Cache Tools

Manage `md5-dict` cache files.

### `cache`

**Subcommands:**

*   `verify [location]`: Verify cache exists for ebuilds.
*   `generate [location]`: Generate cache for ebuilds.
*   `set-method <method>`: Set the cache method in `layout.conf`.
*   `list-methods`: List available cache methods.
*   `clean [location]`: Clean up unused cache entries.

**Example Usage:**

Generate the ebuild cache for the current overlay:

```bash
g2 cache generate .
```

## Package Description Index

Tools relating to generating and verifying `pkg_desc_index`.

### `pkg-desc-index`

**Subcommands:**

*   `generate [location]`: Generate `pkg_desc_index` file from the repository.
*   `verify [location]`: Verify existing `pkg_desc_index` file matches the repository.

**Example Usage:**

Generate the package description index for the current overlay:

```bash
g2 pkg-desc-index generate .
```

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
