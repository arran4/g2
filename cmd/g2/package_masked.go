package main

import (
	"flag"
	"fmt"
	"github.com/arran4/g2"
	"os"
	"strings"
	"time"
)

func (cfg *CmdPackageArgConfig) cmdMasked(args []string) error {
	fs := flag.NewFlagSet("masked", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fmt.Printf("\t\t %s \t\t %s\n", "list", "list masked packages")
		fmt.Printf("\t\t %s \t\t %s\n", "check", "check if a package is masked")
		fmt.Printf("\t\t %s \t\t %s\n", "add", "add a package to masked")
		fmt.Printf("\t\t %s \t\t %s\n", "remove", "remove a package from masked")
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
		return cfg.cmdMaskedList(fs.Args()[1:])
	case "check":
		return cfg.cmdMaskedCheck(fs.Args()[1:])
	case "add":
		return cfg.cmdMaskedAdd(fs.Args()[1:])
	case "remove":
		return cfg.cmdMaskedRemove(fs.Args()[1:])
	case "help", "-help", "--help":
		fs.Usage()
		os.Exit(-1)
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %s", cmd)
	}
	return nil
}

func (cfg *CmdPackageArgConfig) cmdMaskedList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	fileOpt := fs.String("file", "profiles/package.mask", "Path to package.mask file")

	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	data, err := g2.ParsePackageMasked(*fileOpt)
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

func (cfg *CmdPackageArgConfig) cmdMaskedCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	fileOpt := fs.String("file", "profiles/package.mask", "Path to package.mask file")

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

	data, err := g2.ParsePackageMasked(*fileOpt)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Not masked\n")
			return nil
		}
		return fmt.Errorf("parsing %s: %w", *fileOpt, err)
	}

	for _, d := range data {
		for _, entry := range d.Entries {
			if strings.Contains(entry.Package, pkgName) {
				fmt.Printf("Masked\n")
				fmt.Printf("  Reason: %s\n", d.Reason)
				fmt.Printf("  Author: %s <%s> (%s)\n", d.Author, d.AuthorEmail, d.Date)
				return nil
			}
		}
	}

	fmt.Printf("Not masked\n")
	return nil
}

func (cfg *CmdPackageArgConfig) cmdMaskedAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	fileOpt := fs.String("file", "profiles/package.mask", "Path to package.mask file")
	authorOpt := fs.String("author", "G2 User", "Author name")
	emailOpt := fs.String("email", "g2@example.com", "Author email")
	reasonOpt := fs.String("reason", "Masked", "Reason for deprecation")

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

	data, err := g2.ParsePackageMasked(*fileOpt)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("parsing %s: %w", *fileOpt, err)
	}

	currentDate := time.Now().Format("2006-01-02")

	for _, pkg := range fs.Args() {
		data = append(data, g2.PackageMasked{
			Entries: []g2.PackageMaskedEntry{
				{Package: pkg},
			},

			Reason:      *reasonOpt,
			Date:        currentDate,
			Author:      *authorOpt,
			AuthorEmail: *emailOpt,
		})
	}

	f, err := os.Create(*fileOpt)
	if err != nil {
		return fmt.Errorf("creating %s: %w", *fileOpt, err)
	}
	defer func() { _ = f.Close() }()

	if err := g2.SerializePackageMasked(f, data); err != nil {
		return fmt.Errorf("serializing %s: %w", *fileOpt, err)
	}

	fmt.Printf("Added to %s\n", *fileOpt)
	return nil
}

func (cfg *CmdPackageArgConfig) cmdMaskedRemove(args []string) error {
	fs := flag.NewFlagSet("remove", flag.ExitOnError)
	fileOpt := fs.String("file", "profiles/package.mask", "Path to package.mask file")

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

	data, err := g2.ParsePackageMasked(*fileOpt)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("parsing %s: %w", *fileOpt, err)
	}

	var newData []g2.PackageMasked
	for _, d := range data {
		var newEntries []g2.PackageMaskedEntry
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

	if err := g2.SerializePackageMasked(f, newData); err != nil {
		return fmt.Errorf("serializing %s: %w", *fileOpt, err)
	}

	fmt.Printf("Removed from %s\n", *fileOpt)
	return nil
}
