package main

import (
	"flag"
	"fmt"
	"github.com/arran4/g2"
	"os"
    "path/filepath"
	"strings"
)

func (cfg *MainArgConfig) cmdMasks(args []string) error {
	fs := flag.NewFlagSet("masks", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fmt.Printf("\t\t %s \t\t %s\n", "list", "list masked packages")
		fmt.Printf("\t\t %s \t\t %s\n", "mask", "add a package to masked")
		fmt.Printf("\t\t %s \t\t %s\n", "unmask", "remove a package from masked")
		fmt.Printf("\t\t %s \t\t %s\n", "reset", "reset a package mask status")
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
		return cfg.cmdMasksList(fs.Args()[1:])
	case "mask":
		return cfg.cmdMasksMask(fs.Args()[1:])
	case "unmask":
		return cfg.cmdMasksUnmask(fs.Args()[1:])
	case "reset":
		return cfg.cmdMasksReset(fs.Args()[1:])
	case "help", "-help", "--help":
		fs.Usage()
		os.Exit(-1)
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %s", cmd)
	}
	return nil
}

func (cfg *MainArgConfig) cmdMasksList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	configRootOpt := fs.String("config-root", "/etc/portage", "Path to config root")
	reposConfOpt := fs.String("repos-conf", "/etc/portage/repos.conf", "Path to repos.conf")

	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s [filter]\n", strings.Join(cfg.Args, " "))
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	filter := fs.Arg(0)

	// 1. Get packages from /etc/portage/package.mask
	userMasked, err := parsePackageMaskDir(filepath.Join(*configRootOpt, "package.mask"))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("parsing package.mask: %w", err)
	}

	// 2. Get packages from /etc/portage/package.unmask
	userUnmasked, err := parsePackageMaskDir(filepath.Join(*configRootOpt, "package.unmask"))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("parsing package.unmask: %w", err)
	}

    // 3. get overlay paths
    var overlayPaths []string
    if rc, err := g2.ParseReposConf(*reposConfOpt); err == nil {
		for _, f := range rc.Files {
			for _, s := range f.Sections {
				if s.Name == "DEFAULT" || s.Disabled {
					continue
				}
				loc := s.Get("location")
				if loc != "" {
					overlayPaths = append(overlayPaths, loc)
				}
			}
		}
	} else {
        // fallback
		reposDir := "/var/db/repos"
		if entries, err := os.ReadDir(reposDir); err == nil {
            for _, entry := range entries {
                if entry.IsDir() {
                    overlayPaths = append(overlayPaths, filepath.Join(reposDir, entry.Name()))
                }
            }
        }
	}

    // 4. Print masks
	fmt.Printf("=== User Masks ===\n")
    printMasks(userMasked, filter)

	fmt.Printf("\n=== User Unmasks ===\n")
    printMasks(userUnmasked, filter)

    fmt.Printf("\n=== Repo Masks ===\n")
    for _, overlayPath := range overlayPaths {
        repoMaskPath := filepath.Join(overlayPath, "profiles", "package.mask")
        repoMasked, err := g2.ParsePackageMasked(repoMaskPath)
        if err != nil && !os.IsNotExist(err) {
            fmt.Printf("Error parsing %s: %v\n", repoMaskPath, err)
            continue
        }
        if len(repoMasked) > 0 {
            // Check if any match before printing repo header
            hasMatch := false
            for _, d := range repoMasked {
                for _, entry := range d.Entries {
                    if filter == "" || strings.Contains(entry.Package, filter) {
                        hasMatch = true
                        break
                    }
                }
                if hasMatch { break }
            }
            if hasMatch {
                fmt.Printf("\n[%s]\n", filepath.Base(overlayPath))
                printMasks(repoMasked, filter)
            }
        }
    }

	return nil
}

func printMasks(data []g2.PackageMasked, filter string) {
    for _, d := range data {
		for _, entry := range d.Entries {
            if filter == "" || strings.Contains(entry.Package, filter) {
			    fmt.Printf("%s\n", entry.Package)
                if d.Reason != "" {
		            fmt.Printf("  Reason: %s\n", d.Reason)
                }
                if d.Author != "" {
		            fmt.Printf("  Author: %s <%s> (%s)\n", d.Author, d.AuthorEmail, d.Date)
                }
            }
		}
	}
}

func parsePackageMaskDir(path string) ([]g2.PackageMasked, error) {
    var allMasks []g2.PackageMasked

    info, err := os.Stat(path)
    if err != nil {
        return nil, err
    }

    if !info.IsDir() {
        return g2.ParsePackageMasked(path)
    }

    entries, err := os.ReadDir(path)
    if err != nil {
        return nil, err
    }

    for _, entry := range entries {
        if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
            continue
        }
        masks, err := g2.ParsePackageMasked(filepath.Join(path, entry.Name()))
        if err != nil {
            return nil, err
        }
        allMasks = append(allMasks, masks...)
    }

    return allMasks, nil
}

func (cfg *MainArgConfig) cmdMasksMask(args []string) error {
	fs := flag.NewFlagSet("mask", flag.ExitOnError)
	configRootOpt := fs.String("config-root", "/etc/portage", "Path to config root")
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

    pkg := fs.Arg(0)
    targetFile := filepath.Join(*configRootOpt, "package.mask")

    // Create directory if not exists
    if err := os.MkdirAll(*configRootOpt, 0755); err != nil {
        return fmt.Errorf("creating config root: %w", err)
    }

    info, err := os.Stat(targetFile)
    if err == nil && info.IsDir() {
        targetFile = filepath.Join(targetFile, "g2.conf")
    }

    f, err := os.OpenFile(targetFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return fmt.Errorf("opening file: %w", err)
    }
    defer func() { _ = f.Close() }()

    if _, err := fmt.Fprintf(f, "%s\n", pkg); err != nil {
        return fmt.Errorf("writing to file: %w", err)
    }

    fmt.Printf("Added %s to %s\n", pkg, targetFile)
	return nil
}

func (cfg *MainArgConfig) cmdMasksUnmask(args []string) error {
	fs := flag.NewFlagSet("unmask", flag.ExitOnError)
	configRootOpt := fs.String("config-root", "/etc/portage", "Path to config root")
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

    pkg := fs.Arg(0)
    targetFile := filepath.Join(*configRootOpt, "package.unmask")

    // Create directory if not exists
    if err := os.MkdirAll(*configRootOpt, 0755); err != nil {
        return fmt.Errorf("creating config root: %w", err)
    }

    info, err := os.Stat(targetFile)
    if err == nil && info.IsDir() {
        targetFile = filepath.Join(targetFile, "g2.conf")
    }

    f, err := os.OpenFile(targetFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return fmt.Errorf("opening file: %w", err)
    }
    defer func() { _ = f.Close() }()

    if _, err := fmt.Fprintf(f, "%s\n", pkg); err != nil {
        return fmt.Errorf("writing to file: %w", err)
    }

    fmt.Printf("Added %s to %s\n", pkg, targetFile)
	return nil
}

func (cfg *MainArgConfig) cmdMasksReset(args []string) error {
	fs := flag.NewFlagSet("reset", flag.ExitOnError)
	configRootOpt := fs.String("config-root", "/etc/portage", "Path to config root")
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

    pkg := fs.Arg(0)

    // We should probably try to clean up g2.conf files in package.mask and package.unmask
    maskDir := filepath.Join(*configRootOpt, "package.mask")
    unmaskDir := filepath.Join(*configRootOpt, "package.unmask")

    removeFromFile := func(path string) error {
        data, err := g2.ParsePackageMasked(path)
        if err != nil {
            return err
        }
        var newData []g2.PackageMasked
        found := false
        for _, d := range data {
            var newEntries []g2.PackageMaskedEntry
            for _, entry := range d.Entries {
                if entry.Package == pkg {
                    found = true
                } else {
                    newEntries = append(newEntries, entry)
                }
            }
            if len(newEntries) > 0 {
                d.Entries = newEntries
                newData = append(newData, d)
            }
        }

        if found {
            f, err := os.Create(path)
            if err != nil {
                return err
            }
            defer func() { _ = f.Close() }()
            if err := g2.SerializePackageMasked(f, newData); err != nil {
                return err
            }
            fmt.Printf("Removed %s from %s\n", pkg, path)
        }
        return nil
    }

    checkDir := func(path string) error {
        info, err := os.Stat(path)
        if err != nil {
            return nil // Ignore if doesn't exist
        }

        if !info.IsDir() {
            return removeFromFile(path)
        }

        entries, err := os.ReadDir(path)
        if err != nil {
            return err
        }
        for _, entry := range entries {
            if !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
                if err := removeFromFile(filepath.Join(path, entry.Name())); err != nil {
                    return err
                }
            }
        }
        return nil
    }

    if err := checkDir(maskDir); err != nil {
        return fmt.Errorf("resetting mask: %w", err)
    }
    if err := checkDir(unmaskDir); err != nil {
        return fmt.Errorf("resetting unmask: %w", err)
    }

	return nil
}
