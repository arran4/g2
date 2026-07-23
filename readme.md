# g2

A comprehensive, multifaceted command-line toolkit for Gentoo Linux overlays and package management.

## Project overview

`g2` is a powerful suite of tools built to assist developers and maintainers in working with Gentoo packages, overlays, and metadata efficiently.
Designed initially for personal use, it has evolved into a robust project capable of generating static HTML sites representing repository dashboards, indexing packages for rapid client-side search, managing Manifests and checksums automatically, and applying linting checks to ensure code quality within the ebuild ecosystem.

It streamlines workflows by automating repetitive repository administration tasks—like dependency resolution, `pkg_desc_index` generation, and USE flag documentation parsing.

## Useful links

* Repository: https://github.com/arran4/g2
* Go package docs: https://pkg.go.dev/github.com/arran4/g2

## Installation

```bash
go install github.com/arran4/g2/cmd/g2@latest
```

## Quick start

Generate a local static site for your overlay:
```bash
g2 overlay site generate -out my_site_dir .
g2 site serve -port 8080 .
```

Verify ebuild Manifests:
```bash
g2 manifest verify .
```

## Modules & Commands

### `manifest`

The `manifest` command group handles operations related to Manifest files.

**Usage:**

```bash
g2 manifest <subcommand> [flags]
```

**Subcommands:**

#### `upsert-from-url`

Updates or inserts a `Manifest` entry for a file downloaded from a URL. This command streams the file, calculates the specified checksums, and updates the `Manifest` file in the specified directory (or specific file path).

**Usage:**

```bash
g2 manifest [flags] upsert-from-url <url> <filename> <manifestFileOrDir>
```

**Arguments:**

* `<url>`: The HTTP/HTTPS URL of the file to download.
* `<filename>`: The filename to record in the `Manifest` (typically the distfile name).
* `<manifestFileOrDir>`: The path to the `Manifest` file or the directory containing it (e.g., the ebuild directory).

**Flags:**

* `-blake2b` (default: `true`): Calculate BLAKE2B checksum.
* `-blake2s` (default: `false`): Calculate BLAKE2S checksum.
* `-md5` (default: `false`): Calculate MD5 checksum.
* `-rmd160` (default: `false`): Calculate RMD160 checksum.
* `-sha1` (default: `false`): Calculate SHA1 checksum.
* `-sha256` (default: `false`): Calculate SHA256 checksum.
* `-sha3_256` (default: `false`): Calculate SHA3-256 checksum.
* `-sha3_512` (default: `false`): Calculate SHA3-512 checksum.
* `-sha512` (default: `true`): Calculate SHA512 checksum.

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

### `metadata`

Commands relating to modifying `metadata.xml` files.

**Usage:**

```bash
g2 metadata [flags] [location]
```

**Flags:**

* `-force`: Force overwrite if type mismatches or other errors
* `-l, -longdescription <string>`: Set long description
* `-m, -maintainer, -maintainer-add <email[:name[:type]]>`: Add/Update maintainer
* `-maintainer-remove <email>`: Remove maintainer
* `-u, -upstream-id, -upstream-add <type:id>`: Add upstream remote ID
* `-upstream-remove <type:id>`: Remove upstream remote ID
* `-use, -use-add <name:description>`: Add/Update USE flag
* `-use-remove <name>`: Remove USE flag

**Example Usage:**

Add a maintainer to `metadata.xml`:
```bash
g2 metadata -maintainer-add "dev@example.com:Developer Name:person" .
```

Add a USE flag description:
```bash
g2 metadata -use-add "custom-flag:Enables a custom feature" .
```

### `ebuild`

Tools for working with `.ebuild` files.

**Usage:**

```bash
g2 ebuild <subcommand>
```

**Subcommands:**

* `init <template_name>`: Initialize an ebuild from a template.
* `templates`: Manage ebuild templates.
* `sh-parse-to-json <ebuild_file>`: Parse an ebuild using the shell parser and output JSON.
* `as-json <ebuild_file>`: Parse an ebuild using the native parser and output JSON.
* `explain <ebuild_file>`: Output a human-readable summary of an ebuild.
* `check <ebuild_file>`: A lightweight structural validator for ebuild files (alias: lint).
* `deps <ebuild_file>`: Extract and format dependency fields.
* `query <ebuild_file> --key <key> [--format lines]`: Query specific fields from a parsed ebuild.

**Example Usage:**

Query a specific field from an ebuild:
```bash
g2 ebuild query my-package-1.0.ebuild --key SRC_URI --format lines
```

Parse an ebuild natively into JSON format:
```bash
g2 ebuild as-json my-package-1.0.ebuild
```

Initialize a new ebuild from the generic template:
```bash
g2 ebuild init generic my-new-package-1.0.ebuild
```

### `overlay site generate`

Generates a static site for a single overlay.

**Usage:**

```bash
g2 overlay site generate [-out <dir>] [-clear] [<location>]
```

**Arguments:**

* `<location>`: Path to the overlay directory, or a Git URL (defaults to `.`).

**Flags:**

* `-out`: Output directory for the generated site (default: `site_out`).
* `-clear`: Clear output directory before generation.
* `-fast-git-modtime`: Use fast (O(1)) but potentially less reliable go-git file log lookup.
* `-recent-duration`: Duration to consider an update 'recent' (e.g., 3mo, 14d, 72h) (default "3mo").

When the overlay contains news items, the generated news dashboard includes RSS 2.0 and Atom feeds at `news/index.rss` and `news/index.atom`. Aggregated sites also provide combined feeds there, while each repository has its own feeds under `repos/<repository>/news/`.

**Example:**

```bash
g2 overlay site generate -out my_site_dir .
```

### `overlay ebuild`

Tools to modify the overlay with ebuilds.

**Usage:**

```bash
g2 overlay ebuild <subcommand>
```

**Subcommands:**

* `move <from> <to>`: Record a package move in profiles/updates.
* `slotmove <package> <from> <to>`: Record a slot move in profiles/updates.
* `install [-category <string>] <ebuild.ebuild> [overlay_path] [-- <files...>]`: Install an ebuild into the overlay, optionally providing a specific category or additional files for the `files/` directory. Automatically triggers manifest, cache, use desc, and pkg_desc_index generation.

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

* `<repositoriesFile>`: Path or URL to a Gentoo `repositories.xml` file, or `-` for stdin.

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

### `lint`

Checks the repository for errors such as ebuild `IUSE` variables missing in `metadata.xml` and missing `md5-cache` files.

**Usage:**

```bash
g2 lint [<location>] [<target_package>...]
```

**Arguments:**

* `<location>`: Path to the overlay directory (defaults to `.`). Detected automatically if it's a valid repo.
* `<target_package>`: Optional specific packages or categories to lint instead of the entire repository (e.g. `app-misc/foo` or just `foo`).

**Example:**

```bash
g2 lint /var/db/repos/my-overlay
g2 lint . app-misc/foo
g2 lint /var/db/repos/my-overlay app-misc/foo dev-util/bar
```

*(Note: In the future, this command may be split into separate subcommands like `g2 lint repo`, `g2 lint package`, and `g2 lint query` for clarity. It does not currently support full package queries like `<pn>::guru`, `app-misc/foo-v3`, or `>=`)*

### `use`

Manage and discover USE flags, `use.desc`, and `use.local.desc`.

**Usage:**

```bash
g2 use <subcommand>
```

**Subcommands:**

* `discover [location]`: Discover USE flags from ebuilds and `metadata.xml` to regenerate `use.desc`, `use.local.desc`, and `metadata.xml`.
* `desc-add <name> <description>`: Add a USE flag description to `use.desc`.
* `desc-remove <name>`: Remove a USE flag description from `use.desc`.
* `desc-edit <name> <description>`: Edit a USE flag description in `use.desc`.
* `desc-list`: List all USE flag descriptions from `use.desc`.
* `local-desc-add <pkg> <name> <description>`: Add a USE local flag description to `use.local.desc`.
* `local-desc-remove <pkg> <name>`: Remove a USE local flag description from `use.local.desc`.
* `local-desc-edit <pkg> <name> <description>`: Edit a USE local flag description in `use.local.desc`.
* `local-desc-list`: List all USE local flag descriptions from `use.local.desc`.

**Example Usage:**

Discover and populate USE flags automatically:
```bash
g2 use discover .
```

List all local USE descriptions:
```bash
g2 use local-desc-list
```

### `cache`

Manage `md5-dict` cache files.

**Usage:**

```bash
g2 cache <subcommand>
```

**Subcommands:**

* `verify [location]`: Verify cache exists for ebuilds.
* `generate [target-packages...]`: Generate cache for ebuilds. Optionally specify package atoms to only generate cache for them.
* `set-method <method>`: Set the cache method in `layout.conf`.
* `list-methods`: List available cache methods.
* `clean [location]`: Clean up unused cache entries.

**Example:**

Generate the ebuild cache for the current overlay:
```bash
g2 cache generate
```

Generate the ebuild cache for specific packages:
```bash
g2 cache generate app-admin/sudoers-emerge app-test/app
```

### `pkg-desc-index`

Tools relating to generating and verifying `pkg_desc_index`.

**Usage:**

```bash
g2 pkg-desc-index <subcommand>
```

**Subcommands:**

* `generate [location]`: Generate `pkg_desc_index` file from the repository.
* `verify [location]`: Verify existing `pkg_desc_index` file matches the repository.

**Example:**

Generate the package description index for the current overlay:
```bash
g2 pkg-desc-index generate .
```

### `package`

Commands relating to packages and search indexing.

**Usage:**

```bash
g2 package <subcommand>
```

**Subcommands:**

* `search`: Search packages.
* `index-overlay`: Index a single overlay.
* `index-repositories`: Index multiple repositories from an xml file.
* `index`: Index local repositories.
* `update`: Update the local index from a remote zip file.
* `deprecated`: Commands relating to deprecated packages.
* `masked`: Commands relating to masked packages within a repository.

## Masks Command
The `g2 masks` command provides tools for inspecting and modifying user-level and repository-level package mask configuration (`/etc/portage/package.mask` and `package.unmask`).

* `g2 masks list [<filter>]`: List all currently masked and unmasked packages across user config files and repository `profiles/package.mask` entries. An optional filter (like `::guru` or `sci-libs/onnxruntime`) can be provided.
* `g2 masks mask <package>`: Add a package to the user's `package.mask` configuration. If `package.mask` is a directory, it creates or appends to a `g2.conf` file inside it. (e.g. `g2 masks mask sci-libs/onnxruntime::guru`)
* `g2 masks unmask <package>`: Add a package to the user's `package.unmask` configuration. If `package.unmask` is a directory, it creates or appends to a `g2.conf` file inside it. (e.g. `g2 masks unmask sci-libs/onnxruntime::guru`)
* `g2 masks reset <package>`: Remove all mentions of a package from the user's `package.mask` and `package.unmask` configurations, sweeping through configuration directories as well.

### `eclass`

Commands relating to eclasses.

**Usage:**

```bash
g2 eclass <subcommand>
```

**Subcommands:**

* `list`: List available eclasses.
* `install`: Install an eclass from gentoo stable.
* `explain`: Human-readable summary output of an eclass.
* `remove`: Remove an eclass.

### `arch`

Commands relating to architectures.

**Usage:**

```bash
g2 arch <subcommand> [options]
```

**Subcommands:**

* `list-add`: Add or edit an architecture in arch.list.
* `list-remove`: Remove an architecture from arch.list.
* `list-ls`: List all architectures in arch.list.
* `desc-add`: Add or edit an architecture in arches.desc.
* `desc-remove`: Remove an architecture from arches.desc.
* `desc-ls`: List all architectures in arches.desc.

### `profile`

Commands relating to profiles.

**Usage:**

```bash
g2 profile <subcommand>
```

**Subcommands:**

* `list`: List profiles.
* `describe`: Describe a profile.


### `conf`

Commands relating to portage configuration.

**Usage:**

```bash
g2 conf all [--repo <path>] [--profile <profile_path>] [--make-conf <make.conf_path>] [--config-root <config_root_path>]
```
Outputs a complete overview of the system's Portage configuration, parsing variables from `make.conf` and the active profile's `make.defaults` cascade.


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

**Flags:**

*   `-format <string>`: Output format: `text` or `json` (default `text`).
*   `-severity <string>`: Only show warnings of this severity (`Error`, `Warning`, `Notice`, `Info`).
*   `-only-source <string>`: Only show warnings from this source (`g2`, `pkgcheck`).
*   `-only-tag <string>`: Only show warnings with this tag (e.g., `site-quality`, `metadata.xml`).

**Example JSON Output:**

```bash
g2 lint -format=json /var/db/repos/my-overlay
```
### `lint list`

Lists all available lint rules.

**Usage:**

```bash
g2 lint list
```

## Agent Skills

`g2` supports an agent-skill subcommand system to let users install, inspect, update, remove, and manage AI agent skills that teach coding agents (like Claude, Cursor, Copilot) how to use this CLI correctly.

### Installation Syntax

Skills can be installed from a local path:
```bash
g2 skill install ./skills/g2
```

### Scopes and Agents
By default, skills are installed into the `project` scope (the current working directory) under `.agents/skills`.

You can modify the scope and agent target using flags:
```bash
# Install to the global user home directory (~/.agents/skills/g2)
g2 skill install --scope user ./skills/g2

# Install specifically for a given agent tool (e.g. ~/.cursor/skills/g2)
g2 skill install --scope user ./skills/g2 --agent cursor
```

### Managing Skills
List installed skills:
```bash
g2 skill list --scope all
```

Inspect an installed skill's metadata:
```bash
g2 skill inspect g2
```

Update an installed skill (uses source checksums to detect manual modifications):
```bash
g2 skill update g2

# Overwrite local modifications
g2 skill update --force g2
```

Remove a skill:
```bash
g2 skill remove g2
```
