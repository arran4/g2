package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"

	_ "github.com/arran4/g2/lints/ebuild"
	_ "github.com/arran4/g2/lints/md5cache"
	_ "github.com/arran4/g2/lints/metadata"
)

func (cfg *MainArgConfig) cmdLint(args []string) error {

	if len(args) > 0 && args[0] == "list" {
		rules := lints.GetAllRules()
		if len(args) > 1 && args[1] == "--format=json" {
			out, _ := json.MarshalIndent(rules, "", "  ")
			fmt.Println(string(out))
			return nil
		}
		fmt.Println("Available Lint Rules:")
		for _, r := range rules {
			fmt.Printf("- %s [%s] (%s): %s\n", r.ID, r.Severity, r.Source, r.Description)
			if r.URL != "" {
				fmt.Printf("  URL: %s\n", r.URL)
			}
		}
		return nil
	}
	fs := flag.NewFlagSet("lint", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: g2 lint [flags] [<location>] [<target_package>...]\n\n")
		fmt.Printf("  <location>\tOptional path to the overlay directory (defaults to '.'). Detected automatically if it's a valid repo.\n")
		fmt.Printf("  <target_package>\tOptional specific packages or categories to lint instead of the entire repository (e.g. app-misc/foo or just foo).\n\n")
		fmt.Printf("Flags:\n")
		fs.PrintDefaults()
	}
	format := fs.String("format", "text", "Output format: text, json, or github-actions")
	severityFilter := fs.String("severity", "", "Only show warnings of this severity (error, warning, notice, info)")
	sourceFilter := fs.String("only-source", "", "Only show warnings from this source (g2, pkgcheck)")
	tagFilter := fs.String("only-tag", "", "Only show warnings with this tag")

	if err := fs.Parse(args); err != nil {
		return err
	}

	location := "."
	var targetPkgs []string

	if fs.NArg() > 0 {
		firstArg := fs.Arg(0)
		isRepo := false
		if _, err := os.Stat(filepath.Join(firstArg, "profiles")); err == nil {
			isRepo = true
		} else if _, err := os.Stat(filepath.Join(firstArg, "metadata", "layout.conf")); err == nil {
			isRepo = true
		} else if firstArg == "." {
			isRepo = true
		}

		if isRepo {
			location = firstArg
			targetPkgs = fs.Args()[1:]
		} else {
			targetPkgs = fs.Args()
		}
	}

	siteData, err := parseRepo(os.DirFS(location), ".", "Linting", true, nil)
	if err != nil {
		return fmt.Errorf("parsing repo: %w", err)
	}

	var targetMap map[string]bool
	if len(targetPkgs) > 0 {
		targetMap = make(map[string]bool)
		for _, p := range targetPkgs {
			cleanP := filepath.ToSlash(filepath.Clean(p))
			targetMap[cleanP] = true
		}
	}

	hasErrors := false

	var allResults []lints.LintResult

	for _, cat := range siteData.Categories {
		for _, pkg := range cat.Packages {
			if len(targetMap) > 0 {
				qualified := pkg.Category + "/" + pkg.Name
				if !targetMap[qualified] && !targetMap[pkg.Name] && !targetMap[pkg.Category] {
					continue
				}
			}
			pkgCopy := g2.PackageData{
				Name:          pkg.Name,
				Category:      pkg.Category,
				Metadata:      pkg.Metadata,
				MetadataError: pkg.MetadataError,
				Manifest:      pkg.Manifest,
			}
			for _, v := range pkg.Versions {
				pkgCopy.Versions = append(pkgCopy.Versions, g2.VersionData{
					Version:      v.Version,
					Ebuild:       v.Ebuild,
					EbuildRawURL: v.EbuildRawURL,
				})
			}

			lintWarnings := lints.PerformLintingResults(location, &pkgCopy)

			// Filter warnings
			var filteredWarnings []lints.LintResult
			for _, w := range lintWarnings {
				if *severityFilter != "" && !strings.EqualFold(string(w.RuleMetadata.Severity), *severityFilter) {
					continue
				}
				if *sourceFilter != "" && string(w.RuleMetadata.Source) != *sourceFilter {
					continue
				}
				if *tagFilter != "" {
					hasTag := false
					for _, t := range w.RuleMetadata.Tags {
						if t == *tagFilter {
							hasTag = true
							break
						}
					}
					if !hasTag {
						continue
					}
				}
				filteredWarnings = append(filteredWarnings, w)
			}

			for i := range filteredWarnings {
				filteredWarnings[i].Package = pkg.Category + "/" + pkg.Name
			}

			if len(filteredWarnings) > 0 {
				hasErrors = true
				if *format == "text" {
					fmt.Printf("[%s/%s]\n", pkg.Category, pkg.Name)
					for _, w := range filteredWarnings {
						fmt.Printf("  - %s\n", w.Message)
					}
				}
				allResults = append(allResults, filteredWarnings...)
			}
		}
	}

	switch *format {
	case "json":
		out, err := json.MarshalIndent(allResults, "", "  ")
		if err != nil {
			return fmt.Errorf("formatting json: %w", err)
		}
		fmt.Println(string(out))
	case "github-actions":
		printGithubActions(allResults)
	}

	if hasErrors {
		return fmt.Errorf("linting found errors")
	}

	if *format == "text" {
		fmt.Println("Linting passed successfully.")
	}
	return nil
}

func printGithubActions(results []lints.LintResult) {
	for _, res := range results {
		level := "error"
		switch res.RuleMetadata.Severity {
		case lints.SeverityWarning:
			level = "warning"
		case lints.SeverityNotice, lints.SeverityInfo:
			level = "notice" // GitHub actions doesn't have info, map to notice
		}

		msg := escapeGithubActions(res.Message)
		title := escapeGithubProperty(res.RuleMetadata.Title)
		file := res.File
		if file == "" {
			file = res.Package
		}

		var props []string
		if file != "" {
			props = append(props, fmt.Sprintf("file=%s", escapeGithubProperty(file)))
		}
		if res.Line > 0 {
			props = append(props, fmt.Sprintf("line=%d", res.Line))
		}
		if title != "" {
			props = append(props, fmt.Sprintf("title=%s", title)) // don't strictly need to escape if we know title format, but let's be safe
		}

		propStr := ""
		if len(props) > 0 {
			propStr = " " + strings.Join(props, ",")
		}

		fmt.Printf("::%s%s::%s\n", level, propStr, msg)
	}
}

func escapeGithubActions(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")
	return s
}

func escapeGithubProperty(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")
	s = strings.ReplaceAll(s, ":", "%3A")
	s = strings.ReplaceAll(s, ",", "%2C")
	return s
}
