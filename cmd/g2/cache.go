package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
)

// CacheFS interface provides read and write abstraction for testability
type CacheFS interface {
	fs.FS
	MkdirAll(path string, perm os.FileMode) error
	Create(name string) (io.WriteCloser, error)
	Remove(name string) error
	Walk(root string, fn filepath.WalkFunc) error
	Stat(name string) (fs.FileInfo, error)
}

// OsCacheFS is a CacheFS implementation that interacts with the real OS filesystem
type OsCacheFS struct {
	base string
	fs.FS
}

func NewOsCacheFS(base string) *OsCacheFS {
	return &OsCacheFS{
		base: base,
		FS:   os.DirFS(base),
	}
}

func (o *OsCacheFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(filepath.Join(o.base, path), perm)
}

func (o *OsCacheFS) Create(name string) (io.WriteCloser, error) {
	return os.Create(filepath.Join(o.base, name))
}

func (o *OsCacheFS) Remove(name string) error {
	return os.Remove(filepath.Join(o.base, name))
}

func (o *OsCacheFS) Walk(root string, fn filepath.WalkFunc) error {
	return filepath.Walk(filepath.Join(o.base, root), func(path string, info fs.FileInfo, err error) error {
		relPath, _ := filepath.Rel(o.base, path)
		return fn(relPath, info, err)
	})
}

func (o *OsCacheFS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(filepath.Join(o.base, name))
}

func (cfg *MainArgConfig) cmdCache(args []string) error {
	fs := flag.NewFlagSet("cache", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fmt.Printf("\t\t %s \t\t %s\n", "verify", "To verify cache exists for ebuilds")
		fmt.Printf("\t\t %s \t\t %s\n", "generate", "To generate cache for ebuilds")
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

	cfs := NewOsCacheFS(*repoDir)
	return doCacheVerify(cfs, ".")
}

func doCacheVerify(cfs CacheFS, repoDir string) error {
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
		if formats := lc.GetValuesAsSlice("cache-formats"); len(formats) > 0 {
			cacheFormats = formats
		}
	}

	siteData, err := parseRepo(cfs, repoDir, "Cache Verification", false, nil)
	if err != nil {
		return fmt.Errorf("parsing repo: %w", err)
	}

	hasErrors := false

	for _, format := range cacheFormats {
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
	if err := fsFlags.Parse(args); err != nil {
		return err
	}

	cfs := NewOsCacheFS(*repoDir)
	return doCacheGenerate(cfs, ".")
}

func doCacheGenerate(cfs CacheFS, repoDir string) error {
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
		if formats := lc.GetValuesAsSlice("cache-formats"); len(formats) > 0 {
			cacheFormats = formats
		}
	}

	siteData, err := parseRepo(cfs, repoDir, "Cache Generation", false, nil)
	if err != nil {
		return fmt.Errorf("parsing repo: %w", err)
	}

	for _, format := range cacheFormats {
		log.Printf("Generating cache for format: %s", format)

		// For now, we only fully support md5-dict as a known format for generation.
		if format != "md5-dict" {
			log.Printf("Warning: Generation for cache format '%s' might not be fully supported, but we'll generate basic variables.", format)
		}

		for _, cat := range siteData.Categories {
			for _, pkg := range cat.Packages {
				cacheDir := filepath.ToSlash(filepath.Join(repoDir, "metadata", format, pkg.Category))
				if err := cfs.MkdirAll(cacheDir, 0755); err != nil {
					return fmt.Errorf("creating cache directory %s: %w", cacheDir, err)
				}

				for _, ver := range pkg.Versions {
					if ver.Ebuild == nil || ver.Ebuild.Vars == nil {
						continue // skip if not properly parsed
					}

					verCachePath := filepath.ToSlash(filepath.Join(cacheDir, fmt.Sprintf("%s-%s", pkg.Name, ver.Version)))

					f, err := cfs.Create(verCachePath)
					if err != nil {
						return fmt.Errorf("creating cache file %s: %w", verCachePath, err)
					}

					// We write variables directly as K=V. Or K=V... Wait, it's just K=V according to devmanual.
					// e.g. DESCRIPTION=...
					for k, v := range ver.Ebuild.Vars {
						// Don't output variables that are empty to match standard md5-dict
						if v != "" {
							// For multi-line or complex things, we might just write as is
							// We can filter to known metadata keys to avoid noise, but for now we write what ParseEbuild extracted.
							// Important ones: DEPEND, RDEPEND, SLOT, SRC_URI, DESCRIPTION, LICENSE, IUSE, KEYWORDS, EAPI
							if isCacheVariable(k) {
								_, _ = fmt.Fprintf(f, "%s=%s\n", k, v)
							}
						}
					}

					// Add an md5 entry. To calculate _md5_, we need the md5 of the ebuild file.
					ebuildPath := filepath.ToSlash(filepath.Join(repoDir, pkg.Category, pkg.Name, fmt.Sprintf("%s-%s.ebuild", pkg.Name, ver.Version)))
					ebuildContent, err := fs.ReadFile(cfs, ebuildPath)
					if err == nil {
						// eclass handling is omitted for this simple cache generation
						md5sum := fmt.Sprintf("%x", md5.Sum(ebuildContent))
						_, _ = fmt.Fprintf(f, "_md5_=%s\n", md5sum)
					}

					_ = f.Close()
				}
			}
		}
	}

	fmt.Println("Cache generation completed successfully.")
	return nil
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

	cfs := NewOsCacheFS(*repoDir)
	return doCacheClean(cfs, ".")
}

func doCacheClean(cfs CacheFS, repoDir string) error {
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
		if formats := lc.GetValuesAsSlice("cache-formats"); len(formats) > 0 {
			cacheFormats = formats
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

		err = cfs.Walk(formatDir, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
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

func isCacheVariable(key string) bool {
	validKeys := map[string]bool{
		"BDEPEND":        true,
		"DEPEND":         true,
		"DESCRIPTION":    true,
		"EAPI":           true,
		"HOMEPAGE":       true,
		"INHERITED":      true,
		"IUSE":           true,
		"KEYWORDS":       true,
		"LICENSE":        true,
		"PDEPEND":        true,
		"PROPERTIES":     true,
		"PROVIDE":        true,
		"RDEPEND":        true,
		"REQUIRED_USE":   true,
		"RESTRICT":       true,
		"SLOT":           true,
		"SRC_URI":        true,
		"DEFINED_PHASES": true,
	}
	return validKeys[key]
}
