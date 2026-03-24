package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (cfg *MainArgConfig) cmdLint(args []string) error {
	fs := flag.NewFlagSet("lint", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	location := "."
	if fs.NArg() > 0 {
		location = fs.Arg(0)
	}

	siteData, err := parseRepo(location, "Linting")
	if err != nil {
		return fmt.Errorf("parsing repo: %w", err)
	}

	hasErrors := false

	for _, cat := range siteData.Categories {
		for _, pkg := range cat.Packages {
			lintWarnings := performLinting(location, pkg)
			if len(lintWarnings) > 0 {
				hasErrors = true
				fmt.Printf("[%s/%s]\n", pkg.Category, pkg.Name)
				for _, w := range lintWarnings {
					fmt.Printf("  - %s\n", w)
				}
			}
		}
	}

	if hasErrors {
		return fmt.Errorf("linting found errors")
	}

	fmt.Println("Linting passed successfully.")
	return nil
}

type LintRule interface {
	Lint(repoDir string, pkg PackageData) []string
}

var lintRules []LintRule

func RegisterLintRule(rule LintRule) {
	lintRules = append(lintRules, rule)
}

func performLinting(repoDir string, pkg PackageData) []string {
	var warnings []string
	for _, rule := range lintRules {
		warnings = append(warnings, rule.Lint(repoDir, pkg)...)
	}
	return warnings
}

func init() {
	RegisterLintRule(&MetadataLintRule{})
	RegisterLintRule(&IUSELintRule{})
	RegisterLintRule(&MD5CacheLintRule{})
}

type MetadataLintRule struct{}

func (r *MetadataLintRule) Lint(repoDir string, pkg PackageData) []string {
	var warnings []string
	if pkg.Metadata == nil {
		if pkg.MetadataError != nil {
			if os.IsNotExist(pkg.MetadataError) {
				warnings = append(warnings, "metadata.xml is missing. Create one to describe the package.")
			} else {
				warnings = append(warnings, fmt.Sprintf("metadata.xml is invalid: %v. Fix the XML syntax or schema.", pkg.MetadataError))
			}
		} else {
			warnings = append(warnings, "metadata.xml is missing or invalid. Check the file for issues.")
		}
	}
	return warnings
}

type IUSELintRule struct{}

func (r *IUSELintRule) Lint(repoDir string, pkg PackageData) []string {
	var warnings []string
	if pkg.Metadata == nil {
		return warnings
	}

	metaUseFlags := make(map[string]bool)
	for _, use := range pkg.Metadata.Use {
		for _, flag := range use.Flags {
			metaUseFlags[flag.Name] = true
		}
	}

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			iuseRaw := strings.ReplaceAll(ver.Ebuild.Vars["IUSE"], "\"", "")
			if iuseRaw != "" {
				iuseFlags := strings.Fields(iuseRaw)
				for _, f := range iuseFlags {
					f = strings.TrimPrefix(f, "+")
					f = strings.TrimPrefix(f, "-")

					globalFlags := map[string]bool{"test": true, "doc": true, "debug": true}

					if !metaUseFlags[f] && !globalFlags[f] {
						warnings = append(warnings, fmt.Sprintf("Ebuild %s uses IUSE flag '%s' which is not documented in metadata.xml. Add the flag to metadata.xml <use> section.", ver.Version, f))
					}
				}
			}
		}
	}
	return warnings
}

type MD5CacheLintRule struct{}

func (r *MD5CacheLintRule) Lint(repoDir string, pkg PackageData) []string {
	var warnings []string
	cachePath := filepath.Join(repoDir, "metadata", "md5-cache", pkg.Category, pkg.Name)
	for _, ver := range pkg.Versions {
		verCachePath := fmt.Sprintf("%s-%s", cachePath, ver.Version)
		if _, err := os.Stat(verCachePath); os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("Missing md5-cache for version %s. Run 'ebuild <ebuild_file> manifest' or 'egencache' to generate it.", ver.Version))
		}
	}
	return warnings
}
