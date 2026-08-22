package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func (cfg *MainArgConfig) cmdLintChanged(args []string) error {
	fs := flag.NewFlagSet("changed", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: g2 lint changed [flags] [<location>]\n\n")
		fmt.Printf("  <location>\tOptional path to the overlay directory (defaults to '.').\n\n")
		fmt.Printf("Description:\n")
		fmt.Printf("  Lints only Gentoo packages modified in the current Git work tree or recent commits.\n")
		fmt.Printf("  - Working tree, index, and untracked package changes are always considered.\n")
		fmt.Printf("  - Committed changes are compared to the explicit --base if supplied.\n")
		fmt.Printf("  - If --base is omitted, the configured upstream branch is used as the base.\n")
		fmt.Printf("  - If neither an explicit base nor an upstream exists, only local changes are considered.\n\n")
		fmt.Printf("Flags:\n")
		fs.PrintDefaults()
	}
	format := fs.String("format", "text", "Output format: text, json, or github-actions")
	severityFilter := fs.String("severity", "", "Only show warnings of this severity (error, warning, notice, info)")
	sourceFilter := fs.String("only-source", "", "Only show warnings from this source (g2, pkgcheck)")
	tagFilter := fs.String("only-tag", "", "Only show warnings with this tag")
	disableRule := fs.String("disable-rule", "", "Comma-separated list of rule IDs to ignore (case-insensitive)")
	ignoreTag := fs.String("ignore-tag", "", "Comma-separated list of tags to ignore")
	base := fs.String("base", "", "Explicit base commit/ref to diff against. If omitted, uses upstream branch.")

	if err := fs.Parse(args); err != nil {
		return err
	}

	location := "."
	if fs.NArg() > 0 {
		location = fs.Arg(0)
	}

	pkgs, err := getGitModifiedPackagesChanged(location, *base)
	if err != nil {
		return fmt.Errorf("getting modified packages: %w", err)
	}

	if len(pkgs) == 0 {
		if *format == "text" {
			fmt.Println("No modified packages found.")
		}
		return nil
	}

	targetMap := make(map[string]bool)
	for _, p := range pkgs {
		targetMap[p] = true
	}

	return cfg.runLintCore(location, targetMap, nil, *format, *severityFilter, *sourceFilter, *tagFilter, *disableRule, *ignoreTag)
}

func getGitModifiedPackagesChanged(repoDir string, explicitBase string) ([]string, error) {
	// First check if it's a git repo
	chkCmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	chkCmd.Dir = repoDir
	if err := chkCmd.Run(); err != nil {
		return nil, fmt.Errorf("not a git repository or git not installed")
	}

	pkgMap := make(map[string]bool)
	var files []string

	// Helper to extract zero-delimited outputs
	runZCmd := func(args ...string) error {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		out, err := cmd.Output()
		if err == nil {
			for _, f := range strings.Split(string(out), "\x00") {
				if f != "" {
					files = append(files, f)
				}
			}
		}
		return err
	}

	// Unstaged + Staged (tracked) modifications/additions/renames
	if err := runZCmd("git", "diff", "-z", "--name-only", "HEAD"); err != nil {
		return nil, fmt.Errorf("git diff HEAD failed: %w", err)
	}

	// Untracked non-ignored files
	if err := runZCmd("git", "ls-files", "-z", "--others", "--exclude-standard"); err != nil {
		return nil, fmt.Errorf("git ls-files failed: %w", err)
	}

	base := explicitBase
	if base == "" {
		// Changes from commits
		cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "@{u}")
		cmd.Dir = repoDir
		out, err := cmd.Output()
		upstream := strings.TrimSpace(string(out))
		if err == nil && upstream != "" {
			base = upstream
		}
	}

	if base != "" {
		// Check if base is valid
		chkCmd := exec.Command("git", "rev-parse", "--verify", base)
		chkCmd.Dir = repoDir
		if err := chkCmd.Run(); err != nil {
			return nil, fmt.Errorf("invalid base ref: %s", base)
		}

		// Use merge-base semantics (commits on this branch, but not on base branch)
		err := runZCmd("git", "diff", "-z", "--name-only", base+"...HEAD")
		if err != nil {
			return nil, fmt.Errorf("diff failed against base %s: %w", base, err)
		}
	}

	for _, f := range files {
		f = filepath.ToSlash(f)
		parts := strings.Split(f, "/")
		if len(parts) >= 2 {
			cat := parts[0]
			pkg := parts[1]

			// Lightly validate package against the file system to avoid needing to do a full parseRepo
			// A valid Gentoo package is in a category/package directory that actually exists and is a directory
			// containing at least one .ebuild file.
			pkgPath := filepath.Join(repoDir, cat, pkg)
			if stat, err := os.Stat(pkgPath); err == nil && stat.IsDir() {
				// To be a real package, it shouldn't just be an arbitrary directory.
				// Although just being a directory under a valid category is a strong hint,
				// let's ensure it's not a top-level infra directory masking as a category.
				if cat != "metadata" && cat != "profiles" && cat != "eclass" && cat != "licenses" && cat != "scripts" && cat != ".github" {
					// Require at least one .ebuild file for it to be considered a current package target
					entries, _ := os.ReadDir(pkgPath)
					isPkg := false
					for _, e := range entries {
						if strings.HasSuffix(e.Name(), ".ebuild") {
							isPkg = true
							break
						}
					}
					if isPkg {
						pkgMap[cat+"/"+pkg] = true
					}
				}
			}
		}
	}

	var pkgs []string
	for p := range pkgMap {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)

	return pkgs, nil
}
