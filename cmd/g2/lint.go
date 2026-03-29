package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"

	_ "github.com/arran4/g2/lints/ebuild"
	_ "github.com/arran4/g2/lints/md5cache"
	_ "github.com/arran4/g2/lints/metadata"
)

func (cfg *MainArgConfig) cmdLint(args []string) error {
	fs := flag.NewFlagSet("lint", flag.ExitOnError)
	format := fs.String("format", "text", "Output format: text or json")
	severityFilter := fs.String("severity", "", "Only show warnings of this severity (error, warning, notice, info)")
	sourceFilter := fs.String("only-source", "", "Only show warnings from this source (g2, pkgcheck)")
	tagFilter := fs.String("only-tag", "", "Only show warnings with this tag")

	if err := fs.Parse(args); err != nil {
		return err
	}

	location := "."
	if fs.NArg() > 0 {
		location = fs.Arg(0)
	}

	siteData, err := parseRepo(os.DirFS(location), ".", "Linting", true, nil)
	if err != nil {
		return fmt.Errorf("parsing repo: %w", err)
	}

	hasErrors := false

	var allResults []lints.LintResult

	for _, cat := range siteData.Categories {
		for _, pkg := range cat.Packages {
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

	if *format == "json" {
		out, err := json.MarshalIndent(allResults, "", "  ")
		if err != nil {
			return fmt.Errorf("formatting json: %w", err)
		}
		fmt.Println(string(out))
	}

	if hasErrors {
		return fmt.Errorf("linting found errors")
	}

	if *format == "text" {
		fmt.Println("Linting passed successfully.")
	}
	return nil
}
