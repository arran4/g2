package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/arran4/g2"
)

func (cfg *MainArgConfig) cmdArch(args []string) error {
	fs := flag.NewFlagSet("arch", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: %s arch <subcommand> [options]\n", cfg.Args[0])
		fmt.Println("Subcommands:")
		fmt.Println("  list-add      Add or edit an architecture in arch.list")
		fmt.Println("  list-remove   Remove an architecture from arch.list")
		fmt.Println("  list-ls       List all architectures in arch.list")
		fmt.Println("  desc-add      Add or edit an architecture in arches.desc")
		fmt.Println("  desc-remove   Remove an architecture from arches.desc")
		fmt.Println("  desc-ls       List all architectures in arches.desc")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing subcommand")
	}

	subcmd := fs.Arg(0)
	switch subcmd {
	case "list-add":
		return cfg.cmdArchListAdd(fs.Args()[1:])
	case "list-remove":
		return cfg.cmdArchListRemove(fs.Args()[1:])
	case "list-ls":
		return cfg.cmdArchListLs(fs.Args()[1:])
	case "desc-add":
		return cfg.cmdArchesDescAdd(fs.Args()[1:])
	case "desc-remove":
		return cfg.cmdArchesDescRemove(fs.Args()[1:])
	case "desc-ls":
		return cfg.cmdArchesDescLs(fs.Args()[1:])
	default:
		fs.Usage()
		return fmt.Errorf("unknown subcommand: %s", subcmd)
	}
}

func (cfg *MainArgConfig) cmdArchListAdd(args []string) error {
	fs := flag.NewFlagSet("list-add", flag.ExitOnError)
	file := fs.String("file", "profiles/arch.list", "Path to arch.list file")
	archName := fs.String("arch", "", "Architecture name")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *archName == "" {
		return fmt.Errorf("-arch must be provided")
	}

	al, err := g2.ParseArchListFile(*file)
	if err != nil {
		// Create new if it doesn't exist
		al = &g2.ArchList{}
	}

	found := false
	for _, a := range al.Arches {
		if a == *archName {
			found = true
			break
		}
	}

	if !found {
		al.Arches = append(al.Arches, *archName)

		// Write it back manually since we don't have WriteFile on ArchList yet
		// Let's implement it inside the tool
		f, err := os.Create(*file)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		for _, a := range al.Arches {
			_, _ = fmt.Fprintln(f, a)
		}

		log.Printf("Successfully added architecture '%s' to %s", *archName, *file)
	} else {
		log.Printf("Architecture '%s' already exists in %s", *archName, *file)
	}
	return nil
}

func (cfg *MainArgConfig) cmdArchListRemove(args []string) error {
	fs := flag.NewFlagSet("list-remove", flag.ExitOnError)
	file := fs.String("file", "profiles/arch.list", "Path to arch.list file")
	archName := fs.String("arch", "", "Architecture name")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *archName == "" {
		return fmt.Errorf("-arch must be provided")
	}

	al, err := g2.ParseArchListFile(*file)
	if err != nil {
		return fmt.Errorf("parsing arch.list: %w", err)
	}

	var newArches []string
	found := false
	for _, a := range al.Arches {
		if a == *archName {
			found = true
		} else {
			newArches = append(newArches, a)
		}
	}

	if found {
		al.Arches = newArches
		f, err := os.Create(*file)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		for _, a := range al.Arches {
			_, _ = fmt.Fprintln(f, a)
		}
		log.Printf("Successfully removed architecture '%s' from %s", *archName, *file)
	} else {
		log.Printf("Architecture '%s' not found in %s", *archName, *file)
	}

	return nil
}

func (cfg *MainArgConfig) cmdArchListLs(args []string) error {
	fs := flag.NewFlagSet("list-ls", flag.ExitOnError)
	file := fs.String("file", "profiles/arch.list", "Path to arch.list file")

	if err := fs.Parse(args); err != nil {
		return err
	}

	al, err := g2.ParseArchListFile(*file)
	if err != nil {
		return fmt.Errorf("parsing arch.list: %w", err)
	}

	for _, a := range al.Arches {
		fmt.Println(a)
	}

	return nil
}

func (cfg *MainArgConfig) cmdArchesDescAdd(args []string) error {
	fs := flag.NewFlagSet("desc-add", flag.ExitOnError)
	file := fs.String("file", "profiles/arches.desc", "Path to arches.desc file")
	archName := fs.String("arch", "", "Architecture name")
	status := fs.String("status", "stable", "Status (e.g. stable, testing)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *archName == "" {
		return fmt.Errorf("-arch must be provided")
	}

	ad, err := g2.ParseArchesDescFile(*file)
	if err != nil {
		return fmt.Errorf("parsing arches.desc: %w", err)
	}

	if ad.Arches == nil {
		ad.Arches = make(map[string]string)
	}
	ad.Arches[*archName] = *status

	if err := ad.WriteFile(*file); err != nil {
		return fmt.Errorf("writing arches.desc: %w", err)
	}

	log.Printf("Successfully added/edited architecture '%s' with status '%s' in %s", *archName, *status, *file)
	return nil
}

func (cfg *MainArgConfig) cmdArchesDescRemove(args []string) error {
	fs := flag.NewFlagSet("desc-remove", flag.ExitOnError)
	file := fs.String("file", "profiles/arches.desc", "Path to arches.desc file")
	archName := fs.String("arch", "", "Architecture name to remove")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *archName == "" {
		return fmt.Errorf("-arch must be provided")
	}

	ad, err := g2.ParseArchesDescFile(*file)
	if err != nil {
		return fmt.Errorf("parsing arches.desc: %w", err)
	}

	if _, ok := ad.Arches[*archName]; ok {
		delete(ad.Arches, *archName)
		if err := ad.WriteFile(*file); err != nil {
			return fmt.Errorf("writing arches.desc: %w", err)
		}
		log.Printf("Successfully removed architecture '%s' from %s", *archName, *file)
	} else {
		log.Printf("Architecture '%s' not found in %s", *archName, *file)
	}

	return nil
}

func (cfg *MainArgConfig) cmdArchesDescLs(args []string) error {
	fs := flag.NewFlagSet("desc-ls", flag.ExitOnError)
	file := fs.String("file", "profiles/arches.desc", "Path to arches.desc file")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ad, err := g2.ParseArchesDescFile(*file)
	if err != nil {
		return fmt.Errorf("parsing arches.desc: %w", err)
	}

	for k, v := range ad.Arches {
		fmt.Printf("%-15s %s\n", k, v)
	}

	return nil
}
