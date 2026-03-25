package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/arran4/g2"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func (cfg *MainArgConfig) cmdUse(args []string) error {
	fs := flag.NewFlagSet("use", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fmt.Printf("\t\t %s \t\t %s\n", "discover", "Discover USE flags from ebuilds and metadata.xml files to regenerate use.desc, use.local.desc, and metadata.xml.")
		fmt.Printf("\t\t %s \t\t %s\n", "desc-add", "Add a USE flag description to use.desc")
		fmt.Printf("\t\t %s \t\t %s\n", "desc-remove", "Remove a USE flag description from use.desc")
		fmt.Printf("\t\t %s \t\t %s\n", "desc-edit", "Edit a USE flag description in use.desc")
		fmt.Printf("\t\t %s \t\t %s\n", "desc-list", "List all USE flag descriptions from use.desc")
		fmt.Printf("\t\t %s \t\t %s\n", "local-desc-add", "Add a USE local flag description to use.local.desc")
		fmt.Printf("\t\t %s \t\t %s\n", "local-desc-remove", "Remove a USE local flag description from use.local.desc")
		fmt.Printf("\t\t %s \t\t %s\n", "local-desc-edit", "Edit a USE local flag description in use.local.desc")
		fmt.Printf("\t\t %s \t\t %s\n", "local-desc-list", "List all USE local flag descriptions from use.local.desc")
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
	case "desc-add", "desc-edit":
		return cfg.cmdUseDescAdd(fs.Args()[1:])
	case "desc-remove":
		return cfg.cmdUseDescRemove(fs.Args()[1:])
	case "desc-list":
		return cfg.cmdUseDescList(fs.Args()[1:])
	case "local-desc-add", "local-desc-edit":
		return cfg.cmdUseLocalDescAdd(fs.Args()[1:])
	case "local-desc-remove":
		return cfg.cmdUseLocalDescRemove(fs.Args()[1:])
	case "local-desc-list":
		return cfg.cmdUseLocalDescList(fs.Args()[1:])
	case "discover":
		return cfg.cmdUseDiscover(fs.Args()[1:])
	default:
		fs.Usage()
		return fmt.Errorf("unknown subcommand %s", cmd)
	}
}

func (cfg *MainArgConfig) cmdUseDiscover(args []string) error {
	fs := flag.NewFlagSet("discover", flag.ExitOnError)
	repo := fs.String("repo", ".", "Path to gentoo repository")
	useDescFile := fs.String("file-desc", "profiles/use.desc", "Path to use.desc file")
	useLocalDescFile := fs.String("file-local-desc", "profiles/use.local.desc", "Path to use.local.desc file")

	updateDesc := fs.Bool("use-desc", true, "Update the use.desc file")
	updateLocalDesc := fs.Bool("use-local-desc", true, "Update the use.local.desc file")
	updateMetadata := fs.Bool("metadata", true, "Update the metadata.xml files")

	if err := fs.Parse(args); err != nil {
		return err
	}

	var ud *g2.UseDesc
	var uld *g2.UseLocalDesc
	var err error

	if *updateDesc {
		ud, err = g2.ParseUseDescFile(*useDescFile)
		if err != nil {
			return fmt.Errorf("parsing use.desc: %w", err)
		}
		if ud.Flags == nil {
			ud.Flags = make(map[string]string)
		}
	}

	if *updateLocalDesc {
		uld, err = g2.ParseUseLocalDescFile(*useLocalDescFile)
		if err != nil {
			return fmt.Errorf("parsing use.local.desc: %w", err)
		}
		if uld.Flags == nil {
			uld.Flags = make(map[string]map[string]string)
		}
	}

	log.Printf("Discovering USE flags in %s...", *repo)

	addedDesc := 0
	addedLocalDesc := 0
	updatedMetadata := 0

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

		isMetadata := info.Name() == "metadata.xml"
		isEbuild := strings.HasSuffix(info.Name(), ".ebuild")

		if !isMetadata && !isEbuild {
			return nil
		}

		dir := filepath.Dir(path)
		pkgName := filepath.Base(dir)
		catName := filepath.Base(filepath.Dir(dir))

		if catName == "." {
			return nil // Not a valid category/package structure
		}

		fullPkgName := fmt.Sprintf("%s/%s", catName, pkgName)

		if isMetadata {
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
					if *updateDesc {
						if existing, exists := ud.Flags[flag.Name]; !exists || existing == "" {
							ud.Flags[flag.Name] = flag.Text
							addedDesc++
						}
					}
					if *updateLocalDesc {
						if uld.Flags[fullPkgName] == nil {
							uld.Flags[fullPkgName] = make(map[string]string)
						}
						if existing, exists := uld.Flags[fullPkgName][flag.Name]; !exists || existing == "" {
							uld.Flags[fullPkgName][flag.Name] = flag.Text
							addedLocalDesc++
						}
					}
				}
			}
		}

		if isEbuild {
			vars := g2.ParseEbuildVariables(path)
			if vars == nil {
				return nil
			}

			ebuild, parseErr := g2.ParseEbuild(os.DirFS(filepath.Dir(path)), info.Name(), g2.ParseVariables)
			if parseErr != nil {
				return nil
			}

			if iuse, ok := ebuild.Vars["IUSE"]; ok && iuse != "" {
				parsedFlags := g2.ParseIUSE(iuse)

				if *updateMetadata {
					metadataPath := filepath.Join(dir, "metadata.xml")
					var pkgMd *g2.PkgMetadata
					if _, err := os.Stat(metadataPath); err == nil {
						data, parseErr := g2.ParseMetadata(metadataPath)
						if parseErr == nil {
							var typeOk bool
							pkgMd, typeOk = data.(*g2.PkgMetadata)
							if !typeOk {
								pkgMd = nil
							}
						}
					}
					if pkgMd == nil {
						pkgMd = &g2.PkgMetadata{XMLName: xml.Name{Local: "pkgmetadata"}}
					}

					localAdded := 0
					for _, flagName := range parsedFlags {
						targetLang := "en"
						var useBlockIdx = -1
						for i, u := range pkgMd.Use {
							if u.Lang == targetLang || (u.Lang == "" && targetLang == "en") {
								useBlockIdx = i
								break
							}
						}

						if useBlockIdx == -1 {
							pkgMd.Use = append(pkgMd.Use, g2.Use{Lang: targetLang})
							useBlockIdx = len(pkgMd.Use) - 1
						}

						found := false
						for _, f := range pkgMd.Use[useBlockIdx].Flags {
							if f.Name == flagName {
								found = true
								break
							}
						}

						if !found {
							pkgMd.Use[useBlockIdx].Flags = append(pkgMd.Use[useBlockIdx].Flags, g2.Flag{
								Name: flagName,
								Text: "",
							})
							localAdded++
						}
					}

					if localAdded > 0 {
						if err := g2.WriteMetadata(metadataPath, pkgMd); err != nil {
							log.Printf("Error writing metadata for %s: %v", metadataPath, err)
						} else {
							log.Printf("Discovered %d new USE flags for %s", localAdded, metadataPath)
							updatedMetadata += localAdded
						}
					}
				}

				for _, flagName := range parsedFlags {
					if *updateDesc {
						if _, exists := ud.Flags[flagName]; !exists {
							ud.Flags[flagName] = ""
							addedDesc++
						}
					}
					if *updateLocalDesc {
						if uld.Flags[fullPkgName] == nil {
							uld.Flags[fullPkgName] = make(map[string]string)
						}
						if _, exists := uld.Flags[fullPkgName][flagName]; !exists {
							uld.Flags[fullPkgName][flagName] = ""
							addedLocalDesc++
						}
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("walking repository: %w", err)
	}

	if *updateDesc && addedDesc > 0 {
		if err := ud.WriteFile(*useDescFile); err != nil {
			return fmt.Errorf("writing use.desc: %w", err)
		}
		log.Printf("Successfully discovered and updated %d USE flags in %s", addedDesc, *useDescFile)
	} else if *updateDesc {
		log.Printf("No new USE flags discovered for %s", *useDescFile)
	}

	if *updateLocalDesc && addedLocalDesc > 0 {
		if err := uld.WriteFile(*useLocalDescFile); err != nil {
			return fmt.Errorf("writing use.local.desc: %w", err)
		}
		log.Printf("Successfully discovered and updated %d USE local flags in %s", addedLocalDesc, *useLocalDescFile)
	} else if *updateLocalDesc {
		log.Printf("No new USE local flags discovered for %s", *useLocalDescFile)
	}

	if *updateMetadata {
		log.Printf("Successfully updated %d missing USE flags across metadata.xml files", updatedMetadata)
	}

	return nil
}

func (cfg *MainArgConfig) cmdUseDescAdd(args []string) error {
	fs := flag.NewFlagSet("desc-add/edit", flag.ExitOnError)
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
	fs := flag.NewFlagSet("desc-remove", flag.ExitOnError)
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
	fs := flag.NewFlagSet("desc-list", flag.ExitOnError)
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

func (cfg *MainArgConfig) cmdUseLocalDescAdd(args []string) error {
	fs := flag.NewFlagSet("local-desc-add/edit", flag.ExitOnError)
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
	fs := flag.NewFlagSet("local-desc-remove", flag.ExitOnError)
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
	fs := flag.NewFlagSet("local-desc-list", flag.ExitOnError)
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
