package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
)

func (cfg *MainArgConfig) cmdOverlayLicense(args []string) error {
	fs := flag.NewFlagSet("overlay license", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: g2 overlay license <subcommand>\n")
		fmt.Printf("\tlist\t\tList all configured licenses\n")
		fmt.Printf("\tshow <name>\tShow the contents of a license\n")
		fmt.Printf("\tadd <name> <file>\tAdd a new license from a file\n")
		fmt.Printf("\tremove <name>\tRemove a license\n")
		fmt.Printf("\talias\t\tManage license aliases (groups)\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing subcommand for overlay license")
	}

	subcmd := fs.Arg(0)
	switch subcmd {
	case "list":
		return cfg.cmdOverlayLicenseList(fs.Args()[1:])
	case "show":
		return cfg.cmdOverlayLicenseShow(fs.Args()[1:])
	case "add":
		return cfg.cmdOverlayLicenseAdd(fs.Args()[1:])
	case "remove":
		return cfg.cmdOverlayLicenseRemove(fs.Args()[1:])
	case "alias":
		return cfg.cmdOverlayLicenseAlias(fs.Args()[1:])
	default:
		fs.Usage()
		return fmt.Errorf("unknown overlay license subcommand: %s", subcmd)
	}
}

func (cfg *MainArgConfig) cmdOverlayLicenseList(args []string) error {
	licensesDir := "licenses"
	entries, err := os.ReadDir(licensesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading licenses directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			fmt.Println(entry.Name())
		}
	}
	return nil
}

func (cfg *MainArgConfig) cmdOverlayLicenseShow(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: g2 overlay license show <name>")
	}
	name := args[0]
	path := filepath.Join("licenses", name)

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading license %s: %w", name, err)
	}

	fmt.Print(string(content))
	return nil
}

func (cfg *MainArgConfig) cmdOverlayLicenseAdd(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: g2 overlay license add <name> <file>")
	}
	name := args[0]
	file := args[1]

	if err := os.MkdirAll("licenses", 0755); err != nil {
		return fmt.Errorf("creating licenses directory: %w", err)
	}

	path := filepath.Join("licenses", name)

	srcFile, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("opening source file %s: %w", file, err)
	}
	defer func() { _ = srcFile.Close() }()

	destFile, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating license file %s: %w", path, err)
	}
	defer func() { _ = destFile.Close() }()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return fmt.Errorf("copying license content: %w", err)
	}

	log.Printf("Added license %s", name)
	return nil
}

func (cfg *MainArgConfig) cmdOverlayLicenseRemove(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: g2 overlay license remove <name>")
	}
	name := args[0]
	path := filepath.Join("licenses", name)

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing license %s: %w", name, err)
	}

	log.Printf("Removed license %s", name)
	return nil
}

func (cfg *MainArgConfig) cmdOverlayLicenseAlias(args []string) error {
	fs := flag.NewFlagSet("overlay license alias", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: g2 overlay license alias <subcommand>\n")
		fmt.Printf("\tlist\t\tList all license groups/aliases\n")
		fmt.Printf("\tadd <alias> <license...>\tAdd a license alias mapping to target licenses\n")
		fmt.Printf("\tremove <alias>\tRemove a license alias\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing subcommand for overlay license alias")
	}

	subcmd := fs.Arg(0)
	switch subcmd {
	case "list":
		return cfg.cmdOverlayLicenseAliasList(fs.Args()[1:])
	case "add":
		return cfg.cmdOverlayLicenseAliasAdd(fs.Args()[1:])
	case "remove":
		return cfg.cmdOverlayLicenseAliasRemove(fs.Args()[1:])
	default:
		fs.Usage()
		return fmt.Errorf("unknown overlay license alias subcommand: %s", subcmd)
	}
}

func (cfg *MainArgConfig) cmdOverlayLicenseAliasList(args []string) error {
	mappingFile := filepath.Join("profiles", "license_groups")
	f, err := os.Open(mappingFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("opening license groups file: %w", err)
	}
	defer func() { _ = f.Close() }()

	groups, err := g2.ParseLicenseGroups(f)
	if err != nil {
		return fmt.Errorf("parsing license groups: %w", err)
	}

	for group, licenses := range groups {
		fmt.Printf("%s %s\n", group, strings.Join(licenses, " "))
	}
	return nil
}

func (cfg *MainArgConfig) cmdOverlayLicenseAliasAdd(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: g2 overlay license alias add <alias> <license...>")
	}
	alias := args[0]
	licenses := args[1:]

	mappingFile := filepath.Join("profiles", "license_groups")
	if err := os.MkdirAll("profiles", 0755); err != nil {
		return fmt.Errorf("creating profiles directory: %w", err)
	}

	content, err := os.ReadFile(mappingFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading license groups file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	found := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			newLines = append(newLines, line)
			continue
		}

		parts := strings.Fields(trimmedLine)
		if len(parts) >= 1 && parts[0] == alias {
			newLines = append(newLines, fmt.Sprintf("%s %s", alias, strings.Join(licenses, " ")))
			found = true
			continue
		}
		newLines = append(newLines, line)
	}

	if !found {
		if len(newLines) > 0 && newLines[len(newLines)-1] != "" {
			newLines = append(newLines, "")
		}
		newLines = append(newLines, fmt.Sprintf("%s %s", alias, strings.Join(licenses, " ")))
	}

	if err := os.WriteFile(mappingFile, []byte(strings.Join(newLines, "\n")), 0644); err != nil {
		return fmt.Errorf("writing license groups file: %w", err)
	}

	log.Printf("Added license alias %s for licenses %v", alias, licenses)
	return nil
}

func (cfg *MainArgConfig) cmdOverlayLicenseAliasRemove(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: g2 overlay license alias remove <alias>")
	}
	aliasToRemove := args[0]

	mappingFile := filepath.Join("profiles", "license_groups")
	content, err := os.ReadFile(mappingFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Alias %s not found (no license_groups file)", aliasToRemove)
			return nil
		}
		return fmt.Errorf("reading license groups file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	removed := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			newLines = append(newLines, line)
			continue
		}

		parts := strings.Fields(trimmedLine)
		if len(parts) >= 1 && parts[0] == aliasToRemove {
			removed = true
			continue
		}
		newLines = append(newLines, line)
	}

	if !removed {
		log.Printf("Alias %s not found in license groups", aliasToRemove)
		return nil
	}

	// Remove trailing empty line if it became empty because we deleted the last real line
	if len(newLines) > 0 && newLines[len(newLines)-1] == "" && len(newLines) > 1 && newLines[len(newLines)-2] == "" {
		newLines = newLines[:len(newLines)-1]
	}

	if err := os.WriteFile(mappingFile, []byte(strings.Join(newLines, "\n")), 0644); err != nil {
		return fmt.Errorf("writing license groups file: %w", err)
	}

	log.Printf("Removed license alias %s", aliasToRemove)
	return nil
}
