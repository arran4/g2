package main

import (
	"flag"
	"fmt"
	"github.com/arran4/g2"
)

func (cfg *CmdEbuildArgConfig) cmdEbuildDeduplicate(args []string) error {
	fs := flag.NewFlagSet("deduplicate", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	targets := fs.Args()
	if len(targets) == 0 {
		targets = []string{"."}
	}

	removedFiles, err := g2.DeduplicateEbuilds(targets)
	if err != nil {
		return fmt.Errorf("deduplicating ebuilds: %w", err)
	}

	if len(removedFiles) > 0 {
		fmt.Printf("Removed duplicate ebuilds:\n")
		for _, f := range removedFiles {
			fmt.Printf(" - %s\n", f)
		}
	} else {
		fmt.Println("No duplicates found")
	}

	return nil
}
