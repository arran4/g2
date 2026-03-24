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

func (cfg *MainArgConfig) cmdUseDesc(args []string) error {
	fs := flag.NewFlagSet("usedesc", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fmt.Printf("\t\t %s \t\t %s\n", "add", "Add a USE flag description")
		fmt.Printf("\t\t %s \t\t %s\n", "remove", "Remove a USE flag description")
		fmt.Printf("\t\t %s \t\t %s\n", "edit", "Edit a USE flag description")
		fmt.Printf("\t\t %s \t\t %s\n", "list", "List all USE flag descriptions")
		fmt.Printf("\t\t %s \t\t %s\n", "discover", "Discover USE flags from metadata.xml files")
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
		return cfg.cmdUseDescAdd(fs.Args()[1:])
	case "remove":
		return cfg.cmdUseDescRemove(fs.Args()[1:])
	case "edit":
		return cfg.cmdUseDescAdd(fs.Args()[1:]) // Add and edit behave identically for maps
	case "list":
		return cfg.cmdUseDescList(fs.Args()[1:])
	case "discover":
		return cfg.cmdUseDescDiscover(fs.Args()[1:])
	default:
		fs.Usage()
		return fmt.Errorf("unknown subcommand %s", cmd)
	}
}

func (cfg *MainArgConfig) cmdUseDescAdd(args []string) error {
	fs := flag.NewFlagSet("add/edit", flag.ExitOnError)
	file := fs.String("file", "profiles/use.desc", "Path to use.desc file")
	flagName := fs.String("flag", "", "USE flag name")
	desc := fs.String("desc", "", "USE flag description")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *flagName == "" || *desc == "" {
		return fmt.Errorf("both -flag and -desc must be provided")
	}

	ud, err := g2.ParseUseDescFile(*file)
	if err != nil {
		return fmt.Errorf("parsing use.desc: %w", err)
	}

	if ud.Flags == nil {
		ud.Flags = make(map[string]string)
	}
	ud.Flags[*flagName] = *desc

	if err := ud.WriteFile(*file); err != nil {
		return fmt.Errorf("writing use.desc: %w", err)
	}

	log.Printf("Successfully added/edited USE flag '%s'", *flagName)
	return nil
}

func (cfg *MainArgConfig) cmdUseDescRemove(args []string) error {
	fs := flag.NewFlagSet("remove", flag.ExitOnError)
	file := fs.String("file", "profiles/use.desc", "Path to use.desc file")
	flagName := fs.String("flag", "", "USE flag name to remove")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *flagName == "" {
		return fmt.Errorf("-flag must be provided")
	}

	ud, err := g2.ParseUseDescFile(*file)
	if err != nil {
		return fmt.Errorf("parsing use.desc: %w", err)
	}

	if _, ok := ud.Flags[*flagName]; ok {
		delete(ud.Flags, *flagName)
		if err := ud.WriteFile(*file); err != nil {
			return fmt.Errorf("writing use.desc: %w", err)
		}
		log.Printf("Successfully removed USE flag '%s'", *flagName)
	} else {
		log.Printf("USE flag '%s' not found", *flagName)
	}

	return nil
}

func (cfg *MainArgConfig) cmdUseDescList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	file := fs.String("file", "profiles/use.desc", "Path to use.desc file")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ud, err := g2.ParseUseDescFile(*file)
	if err != nil {
		return fmt.Errorf("parsing use.desc: %w", err)
	}

	for k, v := range ud.Flags {
		fmt.Printf("%s - %s\n", k, v)
	}

	return nil
}

func (cfg *MainArgConfig) cmdUseDescDiscover(args []string) error {
	fs := flag.NewFlagSet("discover", flag.ExitOnError)
	file := fs.String("file", "profiles/use.desc", "Path to use.desc file")
	repo := fs.String("repo", ".", "Path to gentoo repository")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ud, err := g2.ParseUseDescFile(*file)
	if err != nil {
		return fmt.Errorf("parsing use.desc: %w", err)
	}

	if ud.Flags == nil {
		ud.Flags = make(map[string]string)
	}

	log.Printf("Discovering USE flags in %s...", *repo)
	foundFlags := make(map[string]string)

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
			metadata, parseErr := g2.ParseMetadata(path)
			if parseErr != nil {
				// Don't log if it's just invalid XML, many non-package metadata.xml files might exist
				return nil
			}

			pkgMd, ok := metadata.(*g2.PkgMetadata)
			if !ok {
				return nil
			}

			for _, useBlock := range pkgMd.Use {
				for _, flag := range useBlock.Flags {
					if existing, exists := foundFlags[flag.Name]; !exists || existing == "" {
						foundFlags[flag.Name] = flag.Text
					}
				}
			}
		}

		if strings.HasSuffix(info.Name(), ".ebuild") {
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

					if _, exists := foundFlags[flagName]; !exists {
						foundFlags[flagName] = "" // No description in ebuild
					}
				}
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("walking repository: %w", err)
	}

	added := 0
	for flagName, desc := range foundFlags {
		if _, exists := ud.Flags[flagName]; !exists {
			ud.Flags[flagName] = desc
			added++
		}
	}

	if added > 0 {
		if err := ud.WriteFile(*file); err != nil {
			return fmt.Errorf("writing use.desc: %w", err)
		}
		log.Printf("Successfully discovered and added %d new USE flags to %s", added, *file)
	} else {
		log.Printf("No new USE flags discovered")
	}

	return nil
}
