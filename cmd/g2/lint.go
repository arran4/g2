package main

import (
	"flag"
	"fmt"

	"github.com/arran4/g2/lints"

	_ "github.com/arran4/g2/lints/ebuild"
	_ "github.com/arran4/g2/lints/md5cache"
	_ "github.com/arran4/g2/lints/metadata"
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

	siteData, err := parseRepo(location, "Linting", true)
	if err != nil {
		return fmt.Errorf("parsing repo: %w", err)
	}

	hasErrors := false

	for _, cat := range siteData.Categories {
		for _, pkg := range cat.Packages {
			pkgCopy := pkg
			lintWarnings := lints.PerformLinting(location, &pkgCopy)
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
