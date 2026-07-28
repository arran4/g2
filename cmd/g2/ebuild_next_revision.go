package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
)

// compareEbuilds compares two ebuild files ignoring lines starting with '#' and whitespace.
func compareEbuilds(file1, file2 string) (bool, error) {
	f1, err := os.Open(file1)
	if err != nil {
		return false, err
	}
	defer f1.Close()

	f2, err := os.Open(file2)
	if err != nil {
		return false, err
	}
	defer f2.Close()

	scanner1 := bufio.NewScanner(f1)
	scanner2 := bufio.NewScanner(f2)

	for {
		var line1, line2 string

		// Advance scanner1 to next significant line
		for scanner1.Scan() {
			t := strings.TrimSpace(scanner1.Text())
			if t != "" && !strings.HasPrefix(t, "#") {
				line1 = t
				break
			}
		}

		// Advance scanner2 to next significant line
		for scanner2.Scan() {
			t := strings.TrimSpace(scanner2.Text())
			if t != "" && !strings.HasPrefix(t, "#") {
				line2 = t
				break
			}
		}

		err1 := scanner1.Err()
		err2 := scanner2.Err()
		if err1 != nil {
			return false, err1
		}
		if err2 != nil {
			return false, err2
		}

		if line1 != line2 {
			return false, nil
		}

		// Both finished
		if line1 == "" && line2 == "" {
			break
		}
	}

	return true, nil
}

func getNextRevision(dir, version string, inspectFile string) (string, int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", -1, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var highestRevGV g2.GentooVersion
	var highestRevFile string
	found := false

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".ebuild") {
			continue
		}

		vars := g2.ParseEbuildVariables(name)
		if vars == nil || vars["PV"] == "" {
			continue
		}

		gv := g2.ParseGentooVersion(vars["PV"])
		origRev := gv.Revision
		gv.Revision = 0
		base := gv.String()
		gv.Revision = origRev

		if base == version {
			if !found || gv.Revision > highestRevGV.Revision {
				highestRevGV = gv
				highestRevFile = filepath.Join(dir, name)
				found = true
			}
		}
	}

	if !found {
		return version, 0, nil
	}

	if inspectFile != "" {
		match, err := compareEbuilds(highestRevFile, inspectFile)
		if err != nil {
			return "", -1, fmt.Errorf("failed to compare ebuilds: %w", err)
		}
		if match {
			// They match. Output current highest and exit 1
			return highestRevGV.String(), 1, nil
		}
	}

	highestRevGV.IncrementRevision()
	return highestRevGV.String(), 0, nil
}

func (cfg *CmdEbuildArgConfig) cmdEbuildNextRevision(args []string) error {
	fs := flag.NewFlagSet("next-revision", flag.ExitOnError)
	inspect := fs.String("inspect", "", "File to compare against the highest existing revision. If contents match (ignoring comments/whitespace), it outputs the current revision and exits 1.")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 2 {
		return fmt.Errorf("usage: g2 ebuild next-revision [--inspect <new_ebuild_file>] <ebuildDir> <version>")
	}

	dir := fs.Arg(0)
	version := fs.Arg(1)

	nextRev, exitCode, err := getNextRevision(dir, version, *inspect)
	if err != nil {
		return err
	}

	fmt.Println(nextRev)
	return &ExitError{Code: exitCode}
}
