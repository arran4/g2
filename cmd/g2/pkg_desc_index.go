package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
)

func (cfg *MainArgConfig) cmdPkgDescIndex(args []string) error {
	fs := flag.NewFlagSet("pkg-desc-index", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fmt.Printf("\t\t %s \t\t %s\n", "generate", "Generate pkg_desc_index file from repository")
		fmt.Printf("\t\t %s \t\t %s\n", "verify", "Verify existing pkg_desc_index file matches repository")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing subcommand")
	}

	cmd := fs.Arg(0)
	cfg.Args = append(cfg.Args, cmd)

	switch cmd {
	case "generate":
		return cfg.cmdPkgDescIndexGenerate(fs.Args()[1:])
	case "verify":
		return cfg.cmdPkgDescIndexVerify(fs.Args()[1:])
	case "help", "-help", "--help":
		fs.Usage()
		os.Exit(0)
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %s", cmd)
	}
	return nil
}

func (cfg *MainArgConfig) cmdPkgDescIndexGenerate(args []string) error {
	fsFlags := flag.NewFlagSet("generate", flag.ExitOnError)
	repoDir := fsFlags.String("repo", ".", "Path to the repository root")
	if err := fsFlags.Parse(args); err != nil {
		return err
	}

	cfs := NewOsCacheFS(*repoDir)
	siteData, err := parseRepo(cfs, ".", "Pkg Desc Index Generation", true, nil)
	if err != nil {
		return fmt.Errorf("parsing repo: %w", err)
	}

	index := &g2.PkgDescIndex{}

	for _, cat := range siteData.Categories {
		for _, pkg := range cat.Packages {
			entry := g2.PkgDescIndexEntry{
				Category: pkg.Category,
				Package:  pkg.Name,
			}

			// description can come from variables
			var desc string
			for _, ver := range pkg.Versions {
				entry.Versions = append(entry.Versions, ver.Version)
				if desc == "" && ver.Ebuild != nil && ver.Ebuild.Vars != nil {
					desc = ver.Ebuild.Vars["DESCRIPTION"]
				}
			}

			// Remove any quotes from description
			desc = strings.Trim(desc, "\"")
			desc = strings.Trim(desc, "'")

			entry.Description = desc
			index.Entries = append(index.Entries, entry)
		}
	}

	index.Sort()

	indexPath := filepath.Join(*repoDir, "metadata", "pkg_desc_index")

	// Ensure metadata directory exists
	if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
		return fmt.Errorf("creating metadata dir: %w", err)
	}

	if err := index.Save(indexPath); err != nil {
		return fmt.Errorf("saving pkg_desc_index: %w", err)
	}

	log.Printf("Successfully generated %s with %d entries", indexPath, len(index.Entries))
	return nil
}

func (cfg *MainArgConfig) cmdPkgDescIndexVerify(args []string) error {
	fsFlags := flag.NewFlagSet("verify", flag.ExitOnError)
	repoDir := fsFlags.String("repo", ".", "Path to the repository root")
	if err := fsFlags.Parse(args); err != nil {
		return err
	}

	indexPath := filepath.Join(*repoDir, "metadata", "pkg_desc_index")
	index, err := g2.ParsePkgDescIndexFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("pkg_desc_index not found at %s. run generate first", indexPath)
		}
		return fmt.Errorf("parsing existing index: %w", err)
	}

	cfs := NewOsCacheFS(*repoDir)
	siteData, err := parseRepo(cfs, ".", "Pkg Desc Index Verification", true, nil)
	if err != nil {
		return fmt.Errorf("parsing repo: %w", err)
	}

	repoPkgs := make(map[string][]string)
	for _, cat := range siteData.Categories {
		for _, pkg := range cat.Packages {
			path := fmt.Sprintf("%s/%s", pkg.Category, pkg.Name)
			var vers []string
			for _, ver := range pkg.Versions {
				vers = append(vers, ver.Version)
			}
			repoPkgs[path] = vers
		}
	}

	hasErrors := false

	// Check if everything in the index is in the repo
	for _, entry := range index.Entries {
		path := fmt.Sprintf("%s/%s", entry.Category, entry.Package)
		repoVers, ok := repoPkgs[path]
		if !ok {
			fmt.Printf("Error: Package %s found in index but missing from repository\n", path)
			hasErrors = true
			continue
		}

		// Simple check for version presence
		versSet := make(map[string]bool)
		for _, v := range repoVers {
			versSet[v] = true
		}

		entryVersSet := make(map[string]bool)
		for _, v := range entry.Versions {
			entryVersSet[v] = true
			if !versSet[v] {
				fmt.Printf("Error: Version %s for %s found in index but missing from repository\n", v, path)
				hasErrors = true
			}
		}

		for _, v := range repoVers {
			if !entryVersSet[v] {
				fmt.Printf("Error: Version %s for %s found in repository but missing from index\n", v, path)
				hasErrors = true
			}
		}
	}

	// Now check if repo packages are missing from the index
	indexPkgs := make(map[string]bool)
	for _, entry := range index.Entries {
		path := fmt.Sprintf("%s/%s", entry.Category, entry.Package)
		indexPkgs[path] = true
	}

	for repoPath := range repoPkgs {
		if !indexPkgs[repoPath] {
			fmt.Printf("Error: Package %s found in repository but missing from index\n", repoPath)
			hasErrors = true
		}
	}

	if hasErrors {
		return fmt.Errorf("pkg_desc_index verification failed")
	}

	log.Println("pkg_desc_index matches repository.")
	return nil
}
