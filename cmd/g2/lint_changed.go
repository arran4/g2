package main

import (
	"flag"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func (cfg *MainArgConfig) cmdLintChanged(args []string) error {
	fs := flag.NewFlagSet("changed", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: g2 lint changed [flags] [<location>]\n\n")
		fmt.Printf("  <location>\tOptional path to the overlay directory (defaults to '.').\n\n")
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
	_ = runZCmd("git", "diff", "-z", "--name-only", "HEAD")

	// Untracked non-ignored files
	_ = runZCmd("git", "ls-files", "-z", "--others", "--exclude-standard")

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
		// typical gentoo package is category/package/...
		// we ignore metadata/layout.conf and profiles/repo_name
		if len(parts) >= 3 && parts[0] != "metadata" && parts[0] != "profiles" {
			pkgMap[parts[0]+"/"+parts[1]] = true
		} else if len(parts) == 2 && parts[0] != "metadata" && parts[0] != "profiles" {
			// A file directly inside a category dir might be weird, but let's be safe.
			// Actually Gentoo packages are cat/pkg/pkg-1.ebuild
			// So len(parts) should be >= 3 to be inside a package.
			// Let's just do len(parts) >= 2 and filter out infrastructure
			pkgMap[parts[0]+"/"+parts[1]] = true
		}
	}

	var pkgs []string
	for p := range pkgMap {
		pkgs = append(pkgs, p)
	}

	return pkgs, nil
}
