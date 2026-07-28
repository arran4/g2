package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/arran4/g2"
)

func checkEbuildExists(dir, version string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".ebuild") {
			continue
		}

		// Use ParseEbuildVariables to safely extract PV based on Gentoo package naming rules
		vars := g2.ParseEbuildVariables(name)
		if vars == nil || vars["PV"] == "" {
			continue
		}

		// Compare base versions. We need to parse PV to remove revision.
		gv := g2.ParseGentooVersion(vars["PV"])
		origRev := gv.Revision
		gv.Revision = 0
		base := gv.String()
		gv.Revision = origRev

		if base == version {
			return true, nil
		}
	}

	return false, nil
}

func (cfg *CmdEbuildArgConfig) cmdEbuildCheckExists(args []string) error {
	fs := flag.NewFlagSet("check-exists", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 2 {
		return fmt.Errorf("usage: g2 ebuild check-exists <ebuildDir> <version>")
	}

	dir := fs.Arg(0)
	version := fs.Arg(1)

	exists, err := checkEbuildExists(dir, version)
	if err != nil {
		return err
	}

	if exists {
		return nil
	} else {
		return &ExitError{Code: 1}
	}
}
