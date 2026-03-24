package main

import (
	"flag"
	"fmt"
	"github.com/arran4/g2"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func (cfg *MainArgConfig) cmdUseLocalDesc(args []string) error {
	fs := flag.NewFlagSet("uselocaldesc", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fmt.Printf("\t\t %s \t\t %s\n", "add", "Add a USE local flag description")
		fmt.Printf("\t\t %s \t\t %s\n", "remove", "Remove a USE local flag description")
		fmt.Printf("\t\t %s \t\t %s\n", "edit", "Edit a USE local flag description")
		fmt.Printf("\t\t %s \t\t %s\n", "list", "List all USE local flag descriptions")
		fmt.Printf("\t\t %s \t\t %s\n", "discover", "Discover USE local flags from metadata.xml files")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing subcommand")
	}

	cmd := fs.Arg(0)
	cfg.Args = append(cfg.Args, cmd)

	switch cmd {
	case "add":
		return cfg.cmdUseLocalDescAdd(fs.Args()[1:])
	case "remove":
		return cfg.cmdUseLocalDescRemove(fs.Args()[1:])
	case "edit":
		return cfg.cmdUseLocalDescAdd(fs.Args()[1:]) // Add and edit behave identically for maps
	case "list":
		return cfg.cmdUseLocalDescList(fs.Args()[1:])
	case "discover":
		return cfg.cmdUseLocalDescDiscover(fs.Args()[1:])
	default:
		fs.Usage()
		return fmt.Errorf("unknown subcommand %s", cmd)
	}
}

func (cfg *MainArgConfig) cmdUseLocalDescAdd(args []string) error {
	fs := flag.NewFlagSet("add/edit", flag.ExitOnError)
	file := fs.String("file", "profiles/use.local.desc", "Path to use.local.desc file")
	pkgName := fs.String("pkg", "", "Package name (e.g., app-admin/conky)")
	flagName := fs.String("flag", "", "USE flag name")
	desc := fs.String("desc", "", "USE flag description")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *pkgName == "" || *flagName == "" || *desc == "" {
		return fmt.Errorf("all -pkg, -flag, and -desc must be provided")
	}

	ud, err := g2.ParseUseLocalDescFile(*file)
	if err != nil {
		return fmt.Errorf("parsing use.local.desc: %w", err)
	}

	if ud.Flags == nil {
		ud.Flags = make(map[string]map[string]string)
	}
	if ud.Flags[*pkgName] == nil {
		ud.Flags[*pkgName] = make(map[string]string)
	}
	ud.Flags[*pkgName][*flagName] = *desc

	if err := ud.WriteFile(*file); err != nil {
		return fmt.Errorf("writing use.local.desc: %w", err)
	}

	log.Printf("Successfully added/edited USE local flag '%s:%s'", *pkgName, *flagName)
	return nil
}

func (cfg *MainArgConfig) cmdUseLocalDescRemove(args []string) error {
	fs := flag.NewFlagSet("remove", flag.ExitOnError)
	file := fs.String("file", "profiles/use.local.desc", "Path to use.local.desc file")
	pkgName := fs.String("pkg", "", "Package name (e.g., app-admin/conky)")
	flagName := fs.String("flag", "", "USE flag name to remove")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *pkgName == "" || *flagName == "" {
		return fmt.Errorf("-pkg and -flag must be provided")
	}

	ud, err := g2.ParseUseLocalDescFile(*file)
	if err != nil {
		return fmt.Errorf("parsing use.local.desc: %w", err)
	}

	if flags, ok := ud.Flags[*pkgName]; ok {
		if _, ok := flags[*flagName]; ok {
			delete(flags, *flagName)
			if len(flags) == 0 {
				delete(ud.Flags, *pkgName)
			}
			if err := ud.WriteFile(*file); err != nil {
				return fmt.Errorf("writing use.local.desc: %w", err)
			}
			log.Printf("Successfully removed USE local flag '%s:%s'", *pkgName, *flagName)
			return nil
		}
	}

	log.Printf("USE local flag '%s:%s' not found", *pkgName, *flagName)

	return nil
}

func (cfg *MainArgConfig) cmdUseLocalDescList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	file := fs.String("file", "profiles/use.local.desc", "Path to use.local.desc file")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ud, err := g2.ParseUseLocalDescFile(*file)
	if err != nil {
		return fmt.Errorf("parsing use.local.desc: %w", err)
	}

	for pkg, flags := range ud.Flags {
		for k, v := range flags {
			fmt.Printf("%s:%s - %s\n", pkg, k, v)
		}
	}

	return nil
}

func (cfg *MainArgConfig) cmdUseLocalDescDiscover(args []string) error {
	fs := flag.NewFlagSet("discover", flag.ExitOnError)
	file := fs.String("file", "profiles/use.local.desc", "Path to use.local.desc file")
	repo := fs.String("repo", ".", "Path to gentoo repository")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ud, err := g2.ParseUseLocalDescFile(*file)
	if err != nil {
		return fmt.Errorf("parsing use.local.desc: %w", err)
	}

	if ud.Flags == nil {
		ud.Flags = make(map[string]map[string]string)
	}

	log.Printf("Discovering USE local flags in %s...", *repo)

	// Since we need to know the package name (category/package), we'll do what site.go does
	// and discover them based on the category/package directory structure.

	added := 0

	err = filepath.Walk(*repo, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "profiles" || info.Name() == "eclass" || strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() == "metadata.xml" {
			// determine category and package name based on directory
			dir := filepath.Dir(path)
			pkgName := filepath.Base(dir)
			catName := filepath.Base(filepath.Dir(dir))

			// Simple heuristic: if catName is ".", this is probably not a package metadata.xml
			if catName == "." {
				return nil
			}

			fullPkgName := fmt.Sprintf("%s/%s", catName, pkgName)

			metadata, parseErr := g2.ParseMetadata(path)
			if parseErr != nil {
				return nil
			}

			pkgMd, ok := metadata.(*g2.PkgMetadata)
			if !ok {
				return nil
			}

			for _, useBlock := range pkgMd.Use {
				for _, flag := range useBlock.Flags {
					if ud.Flags[fullPkgName] == nil {
						ud.Flags[fullPkgName] = make(map[string]string)
					}
					// Only add if it doesn't already exist to respect existing edits
					if existing, exists := ud.Flags[fullPkgName][flag.Name]; !exists || existing == "" {
						ud.Flags[fullPkgName][flag.Name] = flag.Text
						added++
					}
				}
			}
		}

		if strings.HasSuffix(info.Name(), ".ebuild") {
			dir := filepath.Dir(path)
			pkgName := filepath.Base(dir)
			catName := filepath.Base(filepath.Dir(dir))
			if catName == "." {
				return nil
			}
			fullPkgName := fmt.Sprintf("%s/%s", catName, pkgName)

			vars := g2.ParseEbuildVariables(path)
			if vars == nil {
				return nil
			}

			ebuild, parseErr := g2.ParseEbuild(os.DirFS(filepath.Dir(path)), info.Name(), g2.ParseVariables)
			if parseErr != nil {
				return nil
			}

			if iuse, ok := ebuild.Vars["IUSE"]; ok && iuse != "" {
				flags := strings.Fields(iuse)
				for _, flagName := range flags {
					flagName = strings.TrimPrefix(flagName, "+")
					flagName = strings.TrimPrefix(flagName, "-")

					if ud.Flags[fullPkgName] == nil {
						ud.Flags[fullPkgName] = make(map[string]string)
					}
					if _, exists := ud.Flags[fullPkgName][flagName]; !exists {
						ud.Flags[fullPkgName][flagName] = "" // No description in ebuild
						added++
					}
				}
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("walking repository: %w", err)
	}

	if added > 0 {
		if err := ud.WriteFile(*file); err != nil {
			return fmt.Errorf("writing use.local.desc: %w", err)
		}
		log.Printf("Successfully discovered and added %d new USE local flags to %s", added, *file)
	} else {
		log.Printf("No new USE local flags discovered")
	}

	return nil
}
