package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
	"golang.org/x/tools/txtar"
)

func cmdDev() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: g2 dev <subcommand>\nSubcommands: update-txtar-tests")
	}

	subcommand := os.Args[2]
	switch subcommand {
	case "update-txtar-tests":
		return cmdDevUpdateTxtarTests()
	default:
		return fmt.Errorf("unknown subcommand: %s", subcommand)
	}
}

func cmdDevUpdateTxtarTests() error {
	fs := flag.NewFlagSet("update-txtar-tests", flag.ExitOnError)
	if err := fs.Parse(os.Args[3:]); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	targetFile := "testdata/ebuilds.txtar"

	archiveData, err := os.ReadFile(targetFile)
	if err != nil {
		return fmt.Errorf("failed to read txtar archive: %w", err)
	}

	archive := txtar.Parse(archiveData)

	ebuilds := make(map[string][]byte)
	for _, f := range archive.Files {
		if strings.HasSuffix(f.Name, ".ebuild") {
			ebuilds[f.Name] = f.Data
		}
	}

	// Update the files list by re-processing via ParseEbuild
	// Actually we should write a temporary fs or just use the same logic as test

	// Just recreate the files
	for i, f := range archive.Files {
		if strings.HasSuffix(f.Name, ".golden") {
			ebuildName := strings.TrimSuffix(f.Name, ".golden") + ".ebuild"
			ebuildData, ok := ebuilds[ebuildName]
			if !ok {
				continue
			}

			// We can't easily mock an fs.FS here, let's just write to temp file
			tmp, err := os.CreateTemp("", "*.ebuild")
			if err != nil {
				panic(err)
			}
			defer func() { _ = os.Remove(tmp.Name()) }()
			if _, err := tmp.Write(ebuildData); err != nil {
				fmt.Printf("Warning: failed to write temp file for %s: %v\n", ebuildName, err)
				_ = tmp.Close()
				continue
			}
			_ = tmp.Close()

			eb, err := g2.ParseEbuild(os.DirFS(filepath.Dir(tmp.Name())), filepath.Base(tmp.Name()), g2.ParseFull)
			if err != nil {
				fmt.Printf("Warning: failed to parse %s: %v\n", ebuildName, err)
				continue
			}

			// Do not add an extra newline if the generated string doesn't end with one,
			// txtar Format expects strings to have trailing newlines or not based on contents,
			// but eb.String() generally has one.
			newGolden := eb.String()
			if !strings.HasSuffix(newGolden, "\n") {
				newGolden += "\n"
			}
			archive.Files[i].Data = []byte(newGolden)
			fmt.Printf("Updated %s\n", f.Name)
		}
	}

	updatedData := txtar.Format(archive)
	if err := os.WriteFile(targetFile, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write txtar archive: %w", err)
	}

	fmt.Println("Successfully updated txtar test data.")
	return nil
}
