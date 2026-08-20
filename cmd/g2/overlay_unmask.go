package main

import (
	"flag"
	"fmt"
	"github.com/arran4/g2"
	"os"
	"strings"
	"time"
)

func (cfg *MainArgConfig) cmdOverlayUnmask(args []string) error {
	fs := flag.NewFlagSet("overlay unmask", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fmt.Printf("\t\t %s \t\t %s\n", "list", "list unmasked packages")
		fmt.Printf("\t\t %s \t\t %s\n", "check", "check if a package is unmasked")
		fmt.Printf("\t\t %s \t\t %s\n", "add", "add a package to unmasked")
		fmt.Printf("\t\t %s \t\t %s\n", "remove", "remove a package from unmasked")
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing subcommand")
	}

	cmd := fs.Arg(0)
	cfg.Args = append(cfg.Args, cmd)

	switch cmd {
	case "list":
		return cfg.cmdOverlayUnmaskList(fs.Args()[1:])
	case "check":
		return cfg.cmdOverlayUnmaskCheck(fs.Args()[1:])
	case "add":
		return cfg.cmdOverlayUnmaskAdd(fs.Args()[1:])
	case "remove":
		return cfg.cmdOverlayUnmaskRemove(fs.Args()[1:])
	case "help", "-help", "--help":
		fs.Usage()
		return nil
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %s", cmd)
	}
}

func (cfg *MainArgConfig) cmdOverlayUnmaskList(args []string) error {
	fs := flag.NewFlagSet("overlay unmask list", flag.ExitOnError)
	fileOpt := fs.String("file", "profiles/package.unmask", "Path to package.unmask file")

	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	data, err := g2.ParsePackageUnmasked(*fileOpt)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", *fileOpt, err)
	}

	for _, d := range data {
		for _, entry := range d.Entries {
			fmt.Printf("%s\n", entry.Package)
		}
		fmt.Printf("  Reason: %s\n", d.Reason)
		fmt.Printf("  Author: %s <%s> (%s)\n", d.Author, d.AuthorEmail, d.Date)
	}

	return nil
}

func (cfg *MainArgConfig) cmdOverlayUnmaskCheck(args []string) error {
	fs := flag.NewFlagSet("overlay unmask check", flag.ExitOnError)
	fileOpt := fs.String("file", "profiles/package.unmask", "Path to package.unmask file")

	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s <package>\n", strings.Join(cfg.Args, " "))
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing package name")
	}

	pkgName := fs.Arg(0)

	data, err := g2.ParsePackageUnmasked(*fileOpt)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Not unmasked\n")
			return nil
		}
		return fmt.Errorf("parsing %s: %w", *fileOpt, err)
	}

	for _, d := range data {
		for _, entry := range d.Entries {
			if strings.Contains(entry.Package, pkgName) {
				fmt.Printf("Unmasked\n")
				fmt.Printf("  Reason: %s\n", d.Reason)
				fmt.Printf("  Author: %s <%s> (%s)\n", d.Author, d.AuthorEmail, d.Date)
				return nil
			}
		}
	}

	fmt.Printf("Not unmasked\n")
	return nil
}

func (cfg *MainArgConfig) cmdOverlayUnmaskAdd(args []string) error {
	fs := flag.NewFlagSet("overlay unmask add", flag.ExitOnError)
	fileOpt := fs.String("file", "profiles/package.unmask", "Path to package.unmask file")
	authorOpt := fs.String("author", "G2 User", "Author name")
	emailOpt := fs.String("email", "g2@example.com", "Author email")
	reasonOpt := fs.String("reason", "Unmasked", "Reason for deprecation")

	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s <package1> [<package2> ...]\n", strings.Join(cfg.Args, " "))
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing package names")
	}

	data, err := g2.ParsePackageUnmasked(*fileOpt)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("parsing %s: %w", *fileOpt, err)
	}

	currentDate := time.Now().Format("2006-01-02")

	var entries []g2.PackageUnmaskedEntry
	for _, pkg := range fs.Args() {
		entries = append(entries, g2.PackageUnmaskedEntry{Package: pkg})
	}

	data = append(data, g2.PackageUnmasked{
		Entries:     entries,
		Reason:      *reasonOpt,
		Date:        currentDate,
		Author:      *authorOpt,
		AuthorEmail: *emailOpt,
	})

	f, err := os.Create(*fileOpt)
	if err != nil {
		return fmt.Errorf("creating %s: %w", *fileOpt, err)
	}
	defer func() { _ = f.Close() }()

	if err := g2.SerializePackageUnmasked(f, data); err != nil {
		return fmt.Errorf("serializing %s: %w", *fileOpt, err)
	}

	fmt.Printf("Added to %s\n", *fileOpt)
	return nil
}

func (cfg *MainArgConfig) cmdOverlayUnmaskRemove(args []string) error {
	fs := flag.NewFlagSet("overlay unmask remove", flag.ExitOnError)
	fileOpt := fs.String("file", "profiles/package.unmask", "Path to package.unmask file")

	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s <package1> [<package2> ...]\n", strings.Join(cfg.Args, " "))
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing package names")
	}

	toRemove := make(map[string]bool)
	for _, pkg := range fs.Args() {
		toRemove[pkg] = true
	}

	data, err := g2.ParsePackageUnmasked(*fileOpt)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("parsing %s: %w", *fileOpt, err)
	}

	var newData []g2.PackageUnmasked
	for _, d := range data {
		var newEntries []g2.PackageUnmaskedEntry
		for _, entry := range d.Entries {
			if !toRemove[entry.Package] {
				newEntries = append(newEntries, entry)
			}
		}
		if len(newEntries) > 0 {
			d.Entries = newEntries
			newData = append(newData, d)
		}
	}

	f, err := os.Create(*fileOpt)
	if err != nil {
		return fmt.Errorf("creating %s: %w", *fileOpt, err)
	}
	defer func() { _ = f.Close() }()

	if err := g2.SerializePackageUnmasked(f, newData); err != nil {
		return fmt.Errorf("serializing %s: %w", *fileOpt, err)
	}

	fmt.Printf("Removed from %s\n", *fileOpt)
	return nil
}
