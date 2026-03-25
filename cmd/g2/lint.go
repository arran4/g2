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

	siteData, err := parseRepo(os.DirFS(location), ".", "Linting")
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

func performLinting(repoDir string, pkg PackageData) []string {
	var warnings []string

	// Check if metadata exists
	if pkg.Metadata == nil {
		warnings = append(warnings, "metadata.xml is missing or invalid")
	} else {
		// Collect all metadata USE flags
		metaUseFlags := make(map[string]bool)
		for _, use := range pkg.Metadata.Use {
			for _, flag := range use.Flags {
				metaUseFlags[flag.Name] = true
			}
		}

		// Check ebuilds for IUSE missing in metadata.xml
		for _, ver := range pkg.Versions {
			if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
				iuseRaw := strings.ReplaceAll(ver.Ebuild.Vars["IUSE"], "\"", "")
				if iuseRaw != "" {
					iuseFlags := strings.Fields(iuseRaw)
					for _, f := range iuseFlags {
						f = strings.TrimPrefix(f, "+")
						f = strings.TrimPrefix(f, "-")

						// Very basic list of global USE flags we can ignore
						globalFlags := map[string]bool{"test": true, "doc": true, "debug": true}

						if !metaUseFlags[f] && !globalFlags[f] {
							warnings = append(warnings, fmt.Sprintf("Ebuild %s uses IUSE flag '%s' which is not documented in metadata.xml", ver.Version, f))
						}
					}
				}
			}
		}
	}

	// Check for missing md5-cache
	cachePath := filepath.Join(repoDir, "metadata", "md5-cache", pkg.Category, pkg.Name)
	for _, ver := range pkg.Versions {
		verCachePath := fmt.Sprintf("%s-%s", cachePath, ver.Version)
		if _, err := os.Stat(verCachePath); os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("Missing md5-cache for version %s", ver.Version))
		}
	}

	return warnings
}
