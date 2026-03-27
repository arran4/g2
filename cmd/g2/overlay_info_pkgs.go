package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
)

func (cfg *MainArgConfig) cmdOverlayInfoPkgs(args []string) error {
	fs := flag.NewFlagSet("overlay info-pkgs", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: g2 overlay info-pkgs <subcommand>\n")
		fmt.Printf("\tlist\t\t\tList all info packages in the overlay\n")
		fmt.Printf("\tadd <atom>\t\tAdd an info package atom to the overlay\n")
		fmt.Printf("\tremove <atom>\t\tRemove an info package atom from the overlay\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("missing subcommand")
	}

	subcmd := fs.Arg(0)
	switch subcmd {
	case "list":
		return cfg.cmdOverlayInfoPkgsList(fs.Args()[1:])
	case "add":
		return cfg.cmdOverlayInfoPkgsAdd(fs.Args()[1:])
	case "remove":
		return cfg.cmdOverlayInfoPkgsRemove(fs.Args()[1:])
	default:
		fs.Usage()
		return fmt.Errorf("unknown subcommand: %s", subcmd)
	}
}

func getInfoPkgsPath() string {
	return filepath.Join("profiles", "info_pkgs")
}

func (cfg *MainArgConfig) cmdOverlayInfoPkgsList(args []string) error {
	pkgs, err := g2.ParseInfoPkgs(getInfoPkgsPath())
	if err != nil {
		return fmt.Errorf("parsing info_pkgs: %w", err)
	}

	for _, p := range pkgs {
		fmt.Println(p.PackageAtom)
	}
	return nil
}

func (cfg *MainArgConfig) cmdOverlayInfoPkgsAdd(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing package atom to add")
	}
	atom := args[0]

	path := getInfoPkgsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating profiles directory: %w", err)
	}

	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading info_pkgs: %w", err)
	}

	if err == nil {
		// check if it's already there
		pkgs, err := g2.ParseInfoPkgs(path)
		if err != nil {
			return fmt.Errorf("parsing info_pkgs: %w", err)
		}
		for _, p := range pkgs {
			if p.PackageAtom == atom {
				fmt.Printf("Package %s is already in info_pkgs.\n", atom)
				return nil
			}
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("opening info_pkgs file: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Write standard header if it's new/empty
	if len(content) == 0 {
		_, _ = f.WriteString("# Copyright 2004-2025 Gentoo Authors\n")
		_, _ = f.WriteString("# Distributed under the terms of the GNU General Public License v2\n")
		_, _ = f.WriteString("##\n")
		_, _ = f.WriteString("## These ATOMS are printed with a standard 'emerge info' in\n")
		_, _ = f.WriteString("## portage as of 2.0.51-r5. Do not overcrowd the output please.\n")
		_, _ = f.WriteString("##\n")
	} else if len(content) > 0 && content[len(content)-1] != '\n' {
		_, _ = f.WriteString("\n")
	}

	_, _ = f.WriteString(atom + "\n")
	fmt.Printf("Added %s to info_pkgs.\n", atom)
	return nil
}

func (cfg *MainArgConfig) cmdOverlayInfoPkgsRemove(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing package atom to remove")
	}
	atom := args[0]

	path := getInfoPkgsPath()
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Package %s not found in info_pkgs.\n", atom)
			return nil
		}
		return fmt.Errorf("reading info_pkgs: %w", err)
	}

	var newContent bytes.Buffer
	scanner := bufio.NewScanner(bytes.NewReader(content))
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == atom {
			found = true
			continue
		}
		newContent.WriteString(line + "\n")
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanning info_pkgs: %w", err)
	}

	if !found {
		fmt.Printf("Package %s not found in info_pkgs.\n", atom)
		return nil
	}

	if err := os.WriteFile(path, newContent.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing info_pkgs: %w", err)
	}

	fmt.Printf("Removed %s from info_pkgs.\n", atom)
	return nil
}
