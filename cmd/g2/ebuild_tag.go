package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/arran4/g2"
)

func (cfg *CmdEbuildArgConfig) cmdEbuildTag(args []string) error {
	fs := flag.NewFlagSet("tag", flag.ExitOnError)
	dir := fs.String("dir", ".", "Directory to search for ebuilds")
	compare := fs.String("compare", "", "Compare a version to the highest tag (outputs -, =, or +)")
	downgrades := fs.Bool("downgrades", false, "Include downgrades in comparison")
	bumpRevision := fs.Bool("revision", false, "Bump the revision version")
	bumpPatch := fs.Bool("patch", false, "Bump the patch version")
	bumpMinor := fs.Bool("minor", false, "Bump the minor version")
	bumpMajor := fs.Bool("major", false, "Bump the major version")

	if err := fs.Parse(args); err != nil {
		return err
	}

	entries, err := os.ReadDir(*dir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", *dir, err)
	}

	var versions []string
	var pn string

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ebuild") {
			continue
		}

		vars := g2.ParseEbuildVariables(entry.Name())
		if vars == nil || vars["PV"] == "" {
			continue
		}

		if pn == "" {
			pn = vars["PN"]
		} else if pn != vars["PN"] {
			log.Printf("Warning: mixed package names found in directory: %s and %s", pn, vars["PN"])
		}

		versions = append(versions, vars["PV"])
	}

	if len(versions) == 0 {
		return fmt.Errorf("no ebuilds found in %s", *dir)
	}

	// Sort versions using g2.CompareVersions
	sort.Slice(versions, func(i, j int) bool {
		return g2.CompareVersions(versions[i], versions[j]) < 0
	})

	highestVersion := versions[len(versions)-1]

	if *compare != "" {
		comp := g2.CompareVersions(*compare, highestVersion)
		if comp < 0 {
			if *downgrades {
				fmt.Println("-")
			} else {
				log.Printf("Version %s is a downgrade from %s. Ignoring.", *compare, highestVersion)
			}
		} else if comp == 0 {
			fmt.Println("=")
		} else {
			fmt.Println("+")
		}
		return nil
	}

	// Output incremented version if asked
	if *bumpRevision || *bumpPatch || *bumpMinor || *bumpMajor {
		gv := g2.ParseGentooVersion(highestVersion)
		if !gv.IsValid {
			return fmt.Errorf("could not parse version %s for incrementing", highestVersion)
		}

		if *bumpRevision {
			gv.IncrementPart("revision")
		} else if *bumpMajor {
			gv.IncrementPart("major")
		} else if *bumpMinor {
			gv.IncrementPart("minor")
		} else if *bumpPatch {
			gv.IncrementPart("patch")
		}

		fmt.Println(gv.String())
		return nil
	}

	fmt.Println(highestVersion)
	return nil
}
