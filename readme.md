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

### `overlays site generate`

Generates an aggregated static site for multiple remote repositories from a `repositories.xml` file.

**Usage:**

```bash
g2 overlays site generate [-out <dir>] [-clear] <repositoriesFile>
```

**Arguments:**

*   `<repositoriesFile>`: Path or URL to a Gentoo `repositories.xml` file, or `-` for stdin.

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
