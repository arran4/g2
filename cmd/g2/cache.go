package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
)

func (cfg *MainArgConfig) cmdCache(args []string) error {
	fs := flag.NewFlagSet("cache", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fmt.Printf("\t\t %s \t\t %s\n", "verify", "To verify cache exists for ebuilds")
		fmt.Printf("\t\t %s \t %s\n", "generate [packages...]", "To generate cache for ebuilds. Can optionally specify packages to generate.")
		fmt.Printf("\t\t %s \t\t %s\n", "set-method", "To set the cache method in layout.conf")
		fmt.Printf("\t\t %s \t\t %s\n", "list-methods", "To list available cache methods")
		fmt.Printf("\t\t %s \t\t %s\n", "clean", "To clean up unused cache entries")
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
	case "verify":
		return cfg.cmdCacheVerify(fs.Args()[1:])
	case "generate":
		return cfg.cmdCacheGenerate(fs.Args()[1:])
	case "set-method":
		return cfg.cmdCacheSetMethod(fs.Args()[1:])
	case "list-methods":
		return cfg.cmdCacheListMethods(fs.Args()[1:])
	case "clean":
		return cfg.cmdCacheClean(fs.Args()[1:])
	case "help", "-help", "--help":
		fs.Usage()
		os.Exit(0)
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %s", cmd)
	}
	return nil
}

func (cfg *MainArgConfig) cmdCacheVerify(args []string) error {
	fsFlags := flag.NewFlagSet("verify", flag.ExitOnError)
	repoDir := fsFlags.String("repo", ".", "Path to the repository root")
	if err := fsFlags.Parse(args); err != nil {
		return err
	}

	cfs := g2.NewOsCacheFS(*repoDir)
	return doCacheVerify(cfs, ".")
}

func doCacheVerify(cfs g2.CacheFS, repoDir string) error {
	layoutConfPath := filepath.ToSlash(filepath.Join(repoDir, "metadata", "layout.conf"))
	var lc *g2.LayoutConf
	if f, err := cfs.Open(layoutConfPath); err == nil {
		_ = f.Close()
		lc, err = parseLayoutConfFromFS(cfs, layoutConfPath)
		if err != nil {
			log.Printf("Warning: failed to parse layout.conf: %v", err)
			lc = nil
		}
	}

	cacheFormats := []string{"md5-dict"} // Default if not found
	if lc != nil {
		if lc.HasKey("cache-formats") {
			cacheFormats = lc.GetValuesAsSlice("cache-formats")
		}
	}

	siteData, err := parseRepo(cfs, repoDir, "Cache Verification", false, nil)
	if err != nil {
		return fmt.Errorf("parsing repo: %w", err)
	}

	hasErrors := false

	for _, format := range cacheFormats {
		if format != "md5-dict" {
			log.Printf("Warning: Cache format '%s' is not supported. Only md5-dict is supported.", format)
			hasErrors = true
			continue
		}
		log.Printf("Verifying cache for format: %s", format)

		for _, cat := range siteData.Categories {
			for _, pkg := range cat.Packages {
				cachePath := filepath.Join(repoDir, "metadata", format, pkg.Category, pkg.Name)

				for _, ver := range pkg.Versions {
					verCachePath := filepath.ToSlash(fmt.Sprintf("%s-%s", cachePath, ver.Version))
					if _, err := cfs.Stat(verCachePath); os.IsNotExist(err) || err != nil {
						fmt.Printf("Missing %s cache for %s/%s-%s\n", format, pkg.Category, pkg.Name, ver.Version)
						hasErrors = true
					}
				}
			}
		}
	}

	if hasErrors {
		return fmt.Errorf("cache verification found errors")
	}

	fmt.Println("Cache verification passed successfully.")
	return nil
}

func (cfg *MainArgConfig) cmdCacheGenerate(args []string) error {
	fsFlags := flag.NewFlagSet("generate", flag.ExitOnError)
	repoDir := fsFlags.String("repo", ".", "Path to the repository root")
	eclasses := fsFlags.Bool("eclasses", false, "Generate eclasses metadata in cache (off by default)")
	if err := fsFlags.Parse(args); err != nil {
		return err
	}

	cfs := g2.NewOsCacheFS(*repoDir)
	err := g2.GenerateCacheFS(cfs, ".", fsFlags.Args(), *eclasses)
	if err == nil {
		fmt.Println("Cache generation completed successfully.")
	}
	return err
}

func (cfg *MainArgConfig) cmdCacheSetMethod(args []string) error {
	fs := flag.NewFlagSet("set-method", flag.ExitOnError)
	repoDir := fs.String("repo", ".", "Path to the repository root")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: cache set-method <method>")
	}

	method := fs.Arg(0)

	layoutConfPath := filepath.Join(*repoDir, "metadata", "layout.conf")
	var lc *g2.LayoutConf
	var err error

	if _, statErr := os.Stat(layoutConfPath); os.IsNotExist(statErr) {
		lc = &g2.LayoutConf{}
	} else {
		lc, err = g2.ParseLayoutConf(layoutConfPath)
		if err != nil {
			return fmt.Errorf("failed to parse layout.conf: %w", err)
		}
	}

	lc.SetValue("cache-formats", method)

	if err := os.MkdirAll(filepath.Dir(layoutConfPath), 0755); err != nil {
		return fmt.Errorf("creating metadata dir: %w", err)
	}

	if err := g2.WriteLayoutConf(lc, layoutConfPath); err != nil {
		return fmt.Errorf("writing layout.conf: %w", err)
	}

	fmt.Printf("Cache method set to %s\n", method)
	return nil
}

func (cfg *MainArgConfig) cmdCacheListMethods(args []string) error {
	fs := flag.NewFlagSet("list-methods", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Println("Available cache methods:")
	fmt.Println("  md5-dict (default)")
	fmt.Println("  pms (deprecated)")
	return nil
}

func (cfg *MainArgConfig) cmdCacheClean(args []string) error {
	fsFlags := flag.NewFlagSet("clean", flag.ExitOnError)
	repoDir := fsFlags.String("repo", ".", "Path to the repository root")
	if err := fsFlags.Parse(args); err != nil {
		return err
	}

	cfs := g2.NewOsCacheFS(*repoDir)
	return doCacheClean(cfs, ".")
}

func doCacheClean(cfs g2.CacheFS, repoDir string) error {
	layoutConfPath := filepath.ToSlash(filepath.Join(repoDir, "metadata", "layout.conf"))
	var lc *g2.LayoutConf
	if f, err := cfs.Open(layoutConfPath); err == nil {
		_ = f.Close()
		lc, err = parseLayoutConfFromFS(cfs, layoutConfPath)
		if err != nil {
			log.Printf("Warning: failed to parse layout.conf: %v", err)
			lc = nil
		}
	}

	cacheFormats := []string{"md5-dict", "pms"} // check common ones during clean
	if lc != nil {
		if lc.HasKey("cache-formats") {
			cacheFormats = lc.GetValuesAsSlice("cache-formats")
		}
	}

	siteData, err := parseRepo(cfs, repoDir, "Cache Cleaning", false, nil)
	if err != nil {
		return fmt.Errorf("parsing repo: %w", err)
	}

	// build a set of valid ebuild cache paths
	validCacheEntries := make(map[string]bool)

	for _, format := range cacheFormats {
		for _, cat := range siteData.Categories {
			for _, pkg := range cat.Packages {
				for _, ver := range pkg.Versions {
					// cache path format: metadata/md5-dict/sys-apps/pkg-version
					relPath := filepath.Join("metadata", format, pkg.Category, fmt.Sprintf("%s-%s", pkg.Name, ver.Version))
					validCacheEntries[relPath] = true
				}
			}
		}
	}

	cleanedCount := 0

	for _, format := range cacheFormats {
		formatDir := filepath.ToSlash(filepath.Join(repoDir, "metadata", format))
		if _, err := cfs.Stat(formatDir); os.IsNotExist(err) || err != nil {
			continue
		}

		err = cfs.Walk(formatDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			relPath := filepath.ToSlash(path)

			// If it's not a valid cache entry based on current ebuilds, delete it
			if !validCacheEntries[relPath] {
				log.Printf("Removing unused cache entry: %s", relPath)
				if err := cfs.Remove(path); err != nil {
					log.Printf("Failed to remove %s: %v", path, err)
				} else {
					cleanedCount++
				}
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("walking cache dir %s: %w", formatDir, err)
		}
	}

	fmt.Printf("Cleaned %d unused cache entries.\n", cleanedCount)
	return nil
}
