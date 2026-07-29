package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
)

// compareContent compares a file against new content
func compareContent(file1 string, content []byte, ignoreComments bool) (bool, error) {
	f1, err := os.Open(file1)
	if err != nil {
		return false, err
	}
	defer func() { _ = f1.Close() }()

	scanner1 := bufio.NewScanner(f1)
	scanner2 := bufio.NewScanner(bytes.NewReader(content))

	for {
		var line1, line2 string
		var ok1, ok2 bool

		for scanner1.Scan() {
			t := scanner1.Text()
			if ignoreComments {
				if strings.HasPrefix(strings.TrimSpace(t), "#") {
					continue
				}
			}
			line1 = t
			ok1 = true
			break
		}

		for scanner2.Scan() {
			t := scanner2.Text()
			if ignoreComments {
				if strings.HasPrefix(strings.TrimSpace(t), "#") {
					continue
				}
			}
			line2 = t
			ok2 = true
			break
		}

		if err := scanner1.Err(); err != nil {
			return false, err
		}
		if err := scanner2.Err(); err != nil {
			return false, err
		}

		if !ok1 && !ok2 {
			break // Both finished
		}

		if line1 != line2 {
			return false, nil
		}
	}

	return true, nil
}

func (cfg *CmdEbuildArgConfig) cmdEbuildUpsert(args []string) error {
	fs := flag.NewFlagSet("upsert", flag.ExitOnError)
	dirFlag := fs.String("dir", "", "Directory of the ebuilds")
	pkgFlag := fs.String("package", "", "Package name")
	verFlag := fs.String("version", "", "Base version")
	ignoreComments := fs.Bool("ignore-comments", false, "Ignore comments when comparing")

	if err := fs.Parse(args); err != nil {
		return err
	}

	var inputContent []byte
	filename := "-"
	if fs.NArg() > 0 {
		filename = fs.Arg(0)
	}
	var readErr error
	if filename == "-" {
		inputContent, readErr = io.ReadAll(os.Stdin)
	} else {
		inputContent, readErr = os.ReadFile(filename)
	}
	if readErr != nil {
		return fmt.Errorf("failed to read input: %w", readErr)
	}

	var versionToUse string
	if *verFlag != "" {
		versionToUse = *verFlag
	} else {
		parsedVars := g2.ParseEbuildVariablesFromReader(bytes.NewReader(inputContent))
		if parsedVars != nil && parsedVars["PV"] != "" {
			versionToUse = parsedVars["PV"]
		}
	}

	if *dirFlag == "" || *pkgFlag == "" || versionToUse == "" {
		return fmt.Errorf("usage: g2 ebuild upsert --dir <ebuildDir> --package <pkgName> [--version <version>] [--ignore-comments] [new version|version bump type] [filename|-]")
	}

	entries, err := os.ReadDir(*dirFlag)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read directory %s: %w", *dirFlag, err)
	}

	if os.IsNotExist(err) {
		if err := os.MkdirAll(*dirFlag, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", *dirFlag, err)
		}
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
		if vars == nil || vars["PV"] == "" || vars["PN"] != *pkgFlag {
			continue
		}

		gv := g2.ParseGentooVersion(vars["PV"])
		origRev := gv.Revision
		gv.Revision = 0
		base := gv.String()
		gv.Revision = origRev

		if base == versionToUse {
			if !found || gv.Revision > highestRevGV.Revision {
				highestRevGV = gv
				highestRevFile = filepath.Join(*dirFlag, name)
				found = true
			}
		}
	}

	var targetFile string

	if !found {
		// No existing files, write to base
		targetFile = filepath.Join(*dirFlag, fmt.Sprintf("%s-%s.ebuild", *pkgFlag, versionToUse))
	} else {
		// Compare
		match, err := compareContent(highestRevFile, inputContent, *ignoreComments)
		if err != nil {
			return fmt.Errorf("failed to compare contents: %w", err)
		}

		if match {
			fmt.Println(highestRevFile)
			return nil
		}

		// Differ, bump revision
		highestRevGV.IncrementRevision()
		targetFile = filepath.Join(*dirFlag, fmt.Sprintf("%s-%s.ebuild", *pkgFlag, highestRevGV.String()))
	}

	if err := os.WriteFile(targetFile, inputContent, 0644); err != nil {
		return fmt.Errorf("failed to write ebuild file %s: %w", targetFile, err)
	}

	fmt.Println(targetFile)
	return nil
}
