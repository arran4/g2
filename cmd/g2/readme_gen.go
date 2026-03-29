package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

// DocModel represents a documentation node (module, command, or subcommand)
type DocModel struct {
	Name        string
	Description string
	Usage       string
	Flags       []string
	Args        []string
	Subcommands []DocModel
	Examples    []string
}

func (cfg *MainArgConfig) cmdReadmeGen(args []string) error {
	// The current command structure in the codebase is in a transitional phase
	// (partially using standard flags, some custom logic, and preparing for go-subcommand).
	// Because of this, we use an intermediate documentation model.

	readme := `# g2

Some Gentoo CLI tools I wrote for myself.

## Project overview

` + "`g2`" + ` is a suite of tools for working with Gentoo packages, overlays, and metadata.
It helps with fetching distfiles, managing Manifests, building search indexes, and generating a static HTML dashboard representing a repository.

## Useful links

* Repository: https://github.com/arran4/g2
* Go package docs: https://pkg.go.dev/github.com/arran4/g2

## Installation

` + "```bash" + `
go install github.com/arran4/g2/cmd/g2@latest
` + "```" + `

## Quick start

Generate a local static site for your overlay:
` + "```bash" + `
g2 overlay site generate -out my_site_dir .
g2 site serve -port 8080 .
` + "```" + `

Verify ebuild Manifests:
` + "```bash" + `
g2 manifest verify .
` + "```" + `

## Modules & Commands

`

	// Define all documented commands
	models := []DocModel{
		{
			Name:        "manifest",
			Description: "The `manifest` command group handles operations related to Manifest files.",
			Usage:       "g2 manifest <subcommand> [flags]",
			Subcommands: []DocModel{
				{
					Name:        "upsert-from-url",
					Description: "Updates or inserts a `Manifest` entry for a file downloaded from a URL. This command streams the file, calculates the specified checksums, and updates the `Manifest` file in the specified directory (or specific file path).",
					Usage:       "g2 manifest [flags] upsert-from-url <url> <filename> <manifestFileOrDir>",
					Args: []string{
						"`<url>`: The HTTP/HTTPS URL of the file to download.",
						"`<filename>`: The filename to record in the `Manifest` (typically the distfile name).",
						"`<manifestFileOrDir>`: The path to the `Manifest` file or the directory containing it (e.g., the ebuild directory).",
					},
					Flags: []string{
						"`-blake2b` (default: `true`): Calculate BLAKE2B checksum.",
						"`-blake2s` (default: `false`): Calculate BLAKE2S checksum.",
						"`-md5` (default: `false`): Calculate MD5 checksum.",
						"`-rmd160` (default: `false`): Calculate RMD160 checksum.",
						"`-sha1` (default: `false`): Calculate SHA1 checksum.",
						"`-sha256` (default: `false`): Calculate SHA256 checksum.",
						"`-sha3_256` (default: `false`): Calculate SHA3-256 checksum.",
						"`-sha3_512` (default: `false`): Calculate SHA3-512 checksum.",
						"`-sha512` (default: `true`): Calculate SHA512 checksum.",
					},
					Examples: []string{
						"To download a package, calculate SHA256 in addition to defaults, and update the Manifest in the current directory:\n```bash\ng2 manifest -sha256=true upsert-from-url https://example.com/software-1.0.tar.gz software-1.0.tar.gz .\n```",
					},
				},
				{
					Name:        "verify",
					Description: "Verifies the `Manifest` against the actual ebuild files.",
					Usage:       "g2 manifest verify [location]",
				},
				{
					Name:        "clean",
					Description: "Cleans up unused entries from the `Manifest` file.",
					Usage:       "g2 manifest clean [location]",
				},
			},
		},
		{
			Name:        "metadata",
			Description: "Commands relating to modifying `metadata.xml` files.",
			Usage:       "g2 metadata [flags] [location]",
			Flags: []string{
				"`-force`: Force overwrite if type mismatches or other errors",
				"`-l, -longdescription <string>`: Set long description",
				"`-m, -maintainer, -maintainer-add <email[:name[:type]]>`: Add/Update maintainer",
				"`-maintainer-remove <email>`: Remove maintainer",
				"`-u, -upstream-id, -upstream-add <type:id>`: Add upstream remote ID",
				"`-upstream-remove <type:id>`: Remove upstream remote ID",
				"`-use, -use-add <name:description>`: Add/Update USE flag",
				"`-use-remove <name>`: Remove USE flag",
			},
			Examples: []string{
				"Add a maintainer to `metadata.xml`:\n```bash\ng2 metadata -maintainer-add \"dev@example.com:Developer Name:person\" .\n```",
				"Add a USE flag description:\n```bash\ng2 metadata -use-add \"custom-flag:Enables a custom feature\" .\n```",
			},
		},
		{
			Name:        "ebuild",
			Description: "Tools for working with `.ebuild` files.",
			Usage:       "g2 ebuild <subcommand>",
			Subcommands: []DocModel{
				{Name: "init <template_name>", Description: "Initialize an ebuild from a template."},
				{Name: "templates", Description: "Manage ebuild templates."},
				{Name: "sh-parse-to-json <ebuild_file>", Description: "Parse an ebuild using the shell parser and output JSON."},
				{Name: "as-json <ebuild_file>", Description: "Parse an ebuild using the native parser and output JSON."},
			},
			Examples: []string{
				"Parse an ebuild natively into JSON format:\n```bash\ng2 ebuild as-json my-package-1.0.ebuild\n```",
				"Initialize a new ebuild from the generic template:\n```bash\ng2 ebuild init generic my-new-package-1.0.ebuild\n```",
			},
		},
		{
			Name:        "overlay site generate",
			Description: "Generates a static site for a single overlay.",
			Usage:       "g2 overlay site generate [-out <dir>] [-clear] [<location>]",
			Args: []string{
				"`<location>`: Path to the overlay directory, or a Git URL (defaults to `.`).",
			},
			Flags: []string{
				"`-out`: Output directory for the generated site (default: `site_out`).",
				"`-clear`: Clear output directory before generation.",
				"`-fast-git-modtime`: Use fast (O(1)) but potentially less reliable go-git file log lookup.",
				"`-recent-duration`: Duration to consider an update 'recent' (e.g., 3mo, 14d, 72h) (default \"3mo\").",
			},
			Examples: []string{
				"```bash\ng2 overlay site generate -out my_site_dir .\n```",
			},
		},
		{
			Name:        "overlay ebuild",
			Description: "Tools to modify the overlay with ebuilds.",
			Usage:       "g2 overlay ebuild <subcommand>",
			Subcommands: []DocModel{
				{Name: "move <from> <to>", Description: "Record a package move in profiles/updates."},
				{Name: "slotmove <package> <from> <to>", Description: "Record a slot move in profiles/updates."},
				{Name: "install [-category <string>] <ebuild.ebuild> [overlay_path] [-- <files...>]", Description: "Install an ebuild into the overlay, optionally providing a specific category or additional files for the `files/` directory. Automatically triggers manifest, cache, use desc, and pkg_desc_index generation."},
			},
			Examples: []string{
				"Install a package into a specific category with some files:\n```bash\ng2 overlay ebuild install -category sys-apps my-app-1.0.ebuild . -- app.conf app.service\n```",
			},
		},
		{
			Name:        "overlays site generate",
			Description: "Generates an aggregated static site for multiple remote repositories from a `repositories.xml` file.",
			Usage:       "g2 overlays site generate [-out <dir>] [-clear] <repositoriesFile>",
			Args: []string{
				"`<repositoriesFile>`: Path or URL to a Gentoo `repositories.xml` file, or `-` for stdin.",
			},
		},
		{
			Name:        "site serve",
			Description: "Serves the generated static site locally for previewing.",
			Usage:       "g2 site serve [-port <int>] [path_to_overlay]",
			Examples: []string{
				"```bash\ng2 site serve -port 8080 .\n```",
			},
		},
		{
			Name:        "lint",
			Description: "Checks the repository for errors such as ebuild `IUSE` variables missing in `metadata.xml` and missing `md5-cache` files.",
			Usage:       "g2 lint [<location>]",
			Args: []string{
				"`<location>`: Path to the overlay directory (defaults to `.`).",
			},
			Examples: []string{
				"```bash\ng2 lint /var/db/repos/my-overlay\n```",
			},
		},
		{
			Name:        "use",
			Description: "Manage and discover USE flags, `use.desc`, and `use.local.desc`.",
			Usage:       "g2 use <subcommand>",
			Subcommands: []DocModel{
				{Name: "discover [location]", Description: "Discover USE flags from ebuilds and `metadata.xml` to regenerate `use.desc`, `use.local.desc`, and `metadata.xml`."},
				{Name: "desc-add <name> <description>", Description: "Add a USE flag description to `use.desc`."},
				{Name: "desc-remove <name>", Description: "Remove a USE flag description from `use.desc`."},
				{Name: "desc-edit <name> <description>", Description: "Edit a USE flag description in `use.desc`."},
				{Name: "desc-list", Description: "List all USE flag descriptions from `use.desc`."},
				{Name: "local-desc-add <pkg> <name> <description>", Description: "Add a USE local flag description to `use.local.desc`."},
				{Name: "local-desc-remove <pkg> <name>", Description: "Remove a USE local flag description from `use.local.desc`."},
				{Name: "local-desc-edit <pkg> <name> <description>", Description: "Edit a USE local flag description in `use.local.desc`."},
				{Name: "local-desc-list", Description: "List all USE local flag descriptions from `use.local.desc`."},
			},
			Examples: []string{
				"Discover and populate USE flags automatically:\n```bash\ng2 use discover .\n```",
				"List all local USE descriptions:\n```bash\ng2 use local-desc-list\n```",
			},
		},
		{
			Name:        "cache",
			Description: "Manage `md5-dict` cache files.",
			Usage:       "g2 cache <subcommand>",
			Subcommands: []DocModel{
				{Name: "verify [location]", Description: "Verify cache exists for ebuilds."},
				{Name: "generate [location]", Description: "Generate cache for ebuilds."},
				{Name: "set-method <method>", Description: "Set the cache method in `layout.conf`."},
				{Name: "list-methods", Description: "List available cache methods."},
				{Name: "clean [location]", Description: "Clean up unused cache entries."},
			},
			Examples: []string{
				"Generate the ebuild cache for the current overlay:\n```bash\ng2 cache generate .\n```",
			},
		},
		{
			Name:        "pkg-desc-index",
			Description: "Tools relating to generating and verifying `pkg_desc_index`.",
			Usage:       "g2 pkg-desc-index <subcommand>",
			Subcommands: []DocModel{
				{Name: "generate [location]", Description: "Generate `pkg_desc_index` file from the repository."},
				{Name: "verify [location]", Description: "Verify existing `pkg_desc_index` file matches the repository."},
			},
			Examples: []string{
				"Generate the package description index for the current overlay:\n```bash\ng2 pkg-desc-index generate .\n```",
			},
		},
		{
			Name:        "package",
			Description: "Commands relating to packages and search indexing.",
			Usage:       "g2 package <subcommand>",
			Subcommands: []DocModel{
				{Name: "search", Description: "Search packages."},
				{Name: "index-overlay", Description: "Index a single overlay."},
				{Name: "index-repositories", Description: "Index multiple repositories from an xml file."},
				{Name: "index", Description: "Index local repositories."},
				{Name: "update", Description: "Update the local index from a remote zip file."},
			},
		},
	}

	var sb bytes.Buffer
	for _, m := range models {
		sb.WriteString(renderModel(m, 3))
	}

	readme += sb.String()

	// GitHub Action section and Footer
	readme += `
## GitHub Action

You can use the [g2 GitHub Action](https://github.com/arran4/g2-action) to integrate ` + "`g2`" + ` into your CI/CD workflows. It automatically downloads and installs ` + "`g2`" + ` for use in your workflow steps.

### Example Usage

` + "```yaml" + `
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
` + "```" + `

### Search Features
` + "`g2 site`" + ` features a fully integrated browser-side search accessible from the main dashboard.
Search indexes are emitted automatically to ` + "`search/data`" + `.
Users can query package names, descriptions, use flags, and numerous field filters (` + "`category`" + `, ` + "`license`" + `, ` + "`mask`" + `, ` + "`version`" + `, ` + "`depends`" + `, etc.).
Advanced queries support boolean logic (AND, OR, NOT), grouping ` + "`()`" + `, and sequence matching (` + "`'sequence of words'`" + `).
Gentoo version ordering is natively supported for range queries like ` + "`version:>1.2.3`" + `.
`

	if len(args) > 0 {
		return os.WriteFile(args[0], []byte(readme), 0644)
	}
	fmt.Print(readme)
	return nil
}

// renderModel generates markdown for a single module recursively.
// headerLevel starts at 3 for '###'
func renderModel(model DocModel, headerLevel int) string {
	var sb strings.Builder

	h := strings.Repeat("#", headerLevel)
	fmt.Fprintf(&sb, "%s `%s`\n\n", h, model.Name)

	if model.Description != "" {
		fmt.Fprintf(&sb, "%s\n\n", model.Description)
	}

	if model.Usage != "" {
		sb.WriteString("**Usage:**\n\n```bash\n")
		sb.WriteString(model.Usage)
		sb.WriteString("\n```\n\n")
	}

	if len(model.Args) > 0 {
		sb.WriteString("**Arguments:**\n\n")
		for _, arg := range model.Args {
			fmt.Fprintf(&sb, "* %s\n", arg)
		}
		sb.WriteString("\n")
	}

	if len(model.Flags) > 0 {
		sb.WriteString("**Flags:**\n\n")
		for _, f := range model.Flags {
			fmt.Fprintf(&sb, "* %s\n", f)
		}
		sb.WriteString("\n")
	}

	if len(model.Subcommands) > 0 {
		sb.WriteString("**Subcommands:**\n\n")
		// Render subcommands inline as a list if there are no deep flags/args, otherwise render fully
		hasDeepSub := false
		for _, s := range model.Subcommands {
			if len(s.Flags) > 0 || len(s.Args) > 0 || len(s.Examples) > 0 {
				hasDeepSub = true
				break
			}
		}

		if hasDeepSub {
			for _, ssub := range model.Subcommands {
				sb.WriteString(renderModel(ssub, headerLevel+1))
			}
		} else {
			for _, ssub := range model.Subcommands {
				fmt.Fprintf(&sb, "* `%s`: %s\n", ssub.Name, ssub.Description)
			}
			sb.WriteString("\n")
		}
	}

	if len(model.Examples) > 0 {
		if len(model.Examples) == 1 {
			sb.WriteString("**Example:**\n\n")
		} else {
			sb.WriteString("**Example Usage:**\n\n")
		}
		for _, ex := range model.Examples {
			fmt.Fprintf(&sb, "%s\n\n", ex)
		}
	}

	return sb.String()
}
