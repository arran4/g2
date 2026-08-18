package main

import (
	"flag"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func (cfg *MainArgConfig) cmdLintCommits(args []string) error {
	fs := flag.NewFlagSet("commits", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: g2 lint commits [flags] [<location>]\n\n")
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

	if err := fs.Parse(args); err != nil {
		return err
	}

	location := "."
	if fs.NArg() > 0 {
		location = fs.Arg(0)
	}

	pkgs, err := getGitModifiedPackages(location)
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

func getGitModifiedPackages(repoDir string) ([]string, error) {
	pkgMap := make(map[string]bool)
	var files []string

	runCmd := func(args ...string) error {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		out, err := cmd.Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					files = append(files, line)
				}
			}
		}
		return err
	}

	// Uncommitted changes
	runCmd("git", "diff", "--name-only", "HEAD")
	// Staged changes
	runCmd("git", "diff", "--cached", "--name-only")
	// Untracked files
	runCmd("git", "ls-files", "--others", "--exclude-standard")

	// Changes from commits
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "@{u}")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	upstream := strings.TrimSpace(string(out))
	if err != nil || upstream == "" {
		for _, fallback := range []string{"origin/main", "origin/master"} {
			chkCmd := exec.Command("git", "rev-parse", "--verify", fallback)
            chkCmd.Dir = repoDir
			if chkCmd.Run() == nil {
				upstream = fallback
				break
			}
		}
	}

	if upstream != "" {
		runCmd("git", "diff", "--name-only", upstream+"...HEAD")
	} else {
        // If no upstream, get changes from the latest commit as fallback
        runCmd("git", "diff", "--name-only", "HEAD~1...HEAD")
    }

	for _, f := range files {
		f = filepath.ToSlash(f)
		parts := strings.Split(f, "/")
		// typical gentoo package is category/package/...
		if len(parts) >= 2 {
			pkgMap[parts[0]+"/"+parts[1]] = true
		}
	}

	var pkgs []string
	for p := range pkgMap {
		pkgs = append(pkgs, p)
	}

	return pkgs, nil
}
