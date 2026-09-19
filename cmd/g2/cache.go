package main

import (
	"errors"
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
		fmt.Printf("\t\t %s \t\t %s\n", "reconcile", "To idempotently generate, verify and clean cache entries")
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
	case "reconcile":
		return cfg.cmdCacheReconcile(fs.Args()[1:])
	case "help", "-help", "--help":
		fs.Usage()
		return nil
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %s", cmd)
	}
}

func (cfg *MainArgConfig) cmdCacheVerify(args []string) error {
	fsFlags := flag.NewFlagSet("verify", flag.ExitOnError)
	repoDir := fsFlags.String("repo", ".", "Path to the repository root")
	modeStr := fsFlags.String("mode", "ci", "Execution mode: ci or strict")
	masters := fsFlags.String("master-repo", "", "Comma-separated master repository mappings: name=path")
	reposConf := fsFlags.String("repos-conf", "", "Path to Portage repos.conf used to resolve masters")
	if err := fsFlags.Parse(args); err != nil {
		return err
	}

	cfs := g2.NewOsCacheFS(*repoDir)
	policy, err := cachePolicy(*modeStr, *masters, *reposConf)
	if err != nil {
		return err
	}
	return doCacheVerify(cfs, ".", policy)
}

func doCacheVerify(cfs g2.CacheFS, repoDir string, policy *g2.CachePolicy) error {
	layoutConfPath := filepath.ToSlash(filepath.Join(repoDir, "metadata", "layout.conf"))
	var lc *g2.LayoutConf
	if f, err := cfs.Open(layoutConfPath); err == nil {
		_ = f.Close()
		lc, err = parseLayoutConfFromFS(cfs, layoutConfPath)
		if err != nil {
			return fmt.Errorf("failed to parse layout.conf: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to open layout.conf: %w", err)
	}

	cacheFormats := []string{"md5-dict"} // Default if not found
	if lc != nil {
		if lc.HasKey("cache-formats") {
			cacheFormats = lc.GetValuesAsSlice("cache-formats")
		}
	}

	ebuilds, err := discoverCacheEbuilds(cfs, repoDir)
	if err != nil {
		return fmt.Errorf("discovering cache ebuilds: %w", err)
	}

	hasErrors := false
	hasSkipped := false

	eclassResolver, err := g2.BuildEclassResolver(cfs, repoDir, policy)
	if err != nil {
		return fmt.Errorf("building eclass resolver: %w", err)
	}

	for _, format := range cacheFormats {

		if format != "md5-dict" {
			log.Printf("Warning: Cache format '%s' is not supported. Only md5-dict is supported.", format)
			hasErrors = true
			continue
		}
		log.Printf("Verifying cache for format: %s", format)

		// 1. Detect legacy state
		legacyDir := g2.GetLegacyCacheDir(repoDir, format)
		if legacyDir != "" {
			if _, err := cfs.Stat(legacyDir); err == nil {
				fmt.Printf("Warning: Legacy metadata/md5-dict directory exists. Please run cache clean or reconcile.\n")
				hasErrors = true
			} else if !os.IsNotExist(err) {
				log.Printf("Failed to stat legacy directory %s: %v", legacyDir, err)
				hasErrors = true
			}
		}

		validCacheEntries := make(map[string]bool)

		// 2. Check for missing entries and MD5 mismatches
		for _, ebuild := range ebuilds {
			ident := g2.GetCacheIdentity(repoDir, format, ebuild.Category, ebuild.Package, ebuild.Version)
			validCacheEntries[filepath.Clean(ident.CachePath)] = true

			expectedContentStr, status, err := g2.GetExpectedCacheContent(cfs, ident.EbuildPath, ebuild.Version.Ebuild, policy, eclassResolver)
			if err != nil {
				fmt.Printf("Error generating expected cache content for %s: %v\n", ident.CachePath, err)
				hasErrors = true
				continue
			}

			if status == g2.CacheSkipped {
				fmt.Printf("Skipped %s cache for %s/%s-%s: unavailable masters permitted by CI policy: %s\n", format, ident.Category, ident.Package, ident.PVR, strings.Join(eclassResolver.MissingMasters(), ", "))
				hasSkipped = true
				continue
			}

			if status == g2.CacheError {
				fmt.Printf("Error resolving context for %s/%s-%s\n", ident.Category, ident.Package, ident.PVR)
				hasErrors = true
				continue
			}

			result := g2.CompareCacheEntry(cfs, ident.CachePath, expectedContentStr)
			switch result.Status {
			case g2.CacheVerified:
			case g2.CacheDrift:
				fmt.Printf("Cache drift for %s/%s-%s: %s\n", ident.Category, ident.Package, ident.PVR, result.Message)
				hasErrors = true
			case g2.CacheError:
				fmt.Printf("Failed to read cache file %s: %v\n", ident.CachePath, result.Error)
				hasErrors = true
			}
		}

		// 3. Detect orphan/stale entries
		formatDir := g2.GetCacheRoot(repoDir, format)
		if formatDir != "" {
			if _, err := cfs.Stat(formatDir); err == nil {
				err = cfs.Walk(formatDir, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						log.Printf("Walk error at %s: %v", path, err)
						hasErrors = true
						return err
					}
					if d.IsDir() {
						return nil
					}

					found := false
					for validPath := range validCacheEntries {
						if filepath.Clean(validPath) == filepath.Clean(path) {
							found = true
							break
						}
					}
					if !found {
						fmt.Printf("Stale/Orphan cache entry found: %s\n", path)
						hasErrors = true
					}
					return nil
				})
				if err != nil {
					log.Printf("Walk failed on format dir: %v", err)
					hasErrors = true
				}
			} else if !os.IsNotExist(err) {
				log.Printf("Failed to stat cache directory %s: %v", formatDir, err)
				hasErrors = true
			}
		}
	}

	if hasErrors {
		return fmt.Errorf("cache verification found errors")
	}

	if hasSkipped {
		fmt.Println("Cache verification completed with explicitly skipped checks.")
	} else {
		fmt.Println("Cache verification passed successfully.")
	}
	return nil
}

func (cfg *MainArgConfig) cmdCacheGenerate(args []string) error {
	fsFlags := flag.NewFlagSet("generate", flag.ExitOnError)
	repoDir := fsFlags.String("repo", ".", "Path to the repository root")
	modeStr := fsFlags.String("mode", "ci", "Execution mode: ci or strict")
	masters := fsFlags.String("master-repo", "", "Comma-separated master repository mappings: name=path")
	reposConf := fsFlags.String("repos-conf", "", "Path to Portage repos.conf used to resolve masters")
	if err := fsFlags.Parse(args); err != nil {
		return err
	}

	cfs := g2.NewOsCacheFS(*repoDir)
	policy, err := cachePolicy(*modeStr, *masters, *reposConf)
	if err != nil {
		return err
	}
	err = g2.GenerateCacheFS(cfs, ".", fsFlags.Args(), policy)
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
		lc, err = g2.ParseLayoutConfFromReader(f)
		closeErr := f.Close()
		if err != nil {
			return fmt.Errorf("parsing layout.conf %s: %w", layoutConfPath, err)
		}
		if closeErr != nil {
			return fmt.Errorf("closing layout.conf %s: %w", layoutConfPath, closeErr)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("opening layout.conf %s: %w", layoutConfPath, err)
	}

	cacheFormats := []string{"md5-dict", "pms"} // check common ones during clean
	if lc != nil {
		if lc.HasKey("cache-formats") {
			cacheFormats = lc.GetValuesAsSlice("cache-formats")
		}
	}

	ebuilds, err := discoverCacheEbuilds(cfs, repoDir)
	if err != nil {
		return fmt.Errorf("discovering cache ebuilds: %w", err)
	}

	// build a set of valid ebuild cache paths
	validCacheEntries := make(map[string]bool)

	for _, format := range cacheFormats {
		for _, ebuild := range ebuilds {
			ident := g2.GetCacheIdentity(repoDir, format, ebuild.Category, ebuild.Package, ebuild.Version)
			validCacheEntries[filepath.Clean(ident.CachePath)] = true
		}
	}

	cleanedCount := 0

	for _, format := range cacheFormats {
		formatDir := g2.GetCacheRoot(repoDir, format)
		if formatDir == "" {
			log.Printf("Warning: cache format '%s' is not explicitly supported. Skipping.", format)
			continue
		}
		if _, err := cfs.Stat(formatDir); err == nil {
			err = cfs.Walk(formatDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				found := false
				for validPath := range validCacheEntries {
					if filepath.Clean(validPath) == filepath.Clean(path) {
						found = true
						break
					}
				}
				if !found {
					log.Printf("Removing unused cache entry: %s", path)
					if err := cfs.Remove(path); err != nil {
						log.Printf("Failed to remove %s: %v", path, err)
						return fmt.Errorf("removing cache entry %s: %w", path, err)
					} else {
						cleanedCount++
					}
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("walking cache dir %s: %w", formatDir, err)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat cache dir %s: %w", formatDir, err)
		}

		// Explicit legacy cleanup
		legacyDir := g2.GetLegacyCacheDir(repoDir, format)
		if legacyDir != "" {
			if _, err := cfs.Stat(legacyDir); err == nil {
				log.Printf("Removing legacy metadata/md5-dict directory")
				err = cfs.Walk(legacyDir, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if !d.IsDir() {
						if err := cfs.Remove(path); err != nil {
							log.Printf("Failed to remove legacy cache entry %s: %v", path, err)
							return fmt.Errorf("removing legacy cache entry %s: %w", path, err)
						} else {
							cleanedCount++
						}
					}
					return nil
				})
				if err != nil {
					return fmt.Errorf("walking legacy dir %s: %w", legacyDir, err)
				}
				if err := cfs.RemoveAll(legacyDir); err != nil {
					log.Printf("Failed to remove legacy directory %s: %v", legacyDir, err)
					return fmt.Errorf("removing legacy dir %s: %w", legacyDir, err)
				}
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("stat legacy dir %s: %w", legacyDir, err)
			}
		}
	}

	fmt.Printf("Cleaned %d unused cache entries.\n", cleanedCount)
	return nil
}

func (cfg *MainArgConfig) cmdCacheReconcile(args []string) error {
	fsFlags := flag.NewFlagSet("reconcile", flag.ExitOnError)
	repoDir := fsFlags.String("repo", ".", "Path to the repository root")
	modeStr := fsFlags.String("mode", "ci", "Execution mode: ci or strict")
	masters := fsFlags.String("master-repo", "", "Comma-separated master repository mappings: name=path")
	reposConf := fsFlags.String("repos-conf", "", "Path to Portage repos.conf used to resolve masters")
	if err := fsFlags.Parse(args); err != nil {
		return err
	}

	cfs := g2.NewOsCacheFS(*repoDir)
	policy, err := cachePolicy(*modeStr, *masters, *reposConf)
	if err != nil {
		return err
	}
	return doCacheReconcile(cfs, ".", policy)
}

func cachePolicy(mode, masters, reposConf string) (*g2.CachePolicy, error) {
	policy := g2.NewCachePolicy(g2.CacheMode(mode))
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	policy.ReposConfPath = reposConf
	if masters == "" {
		return policy, nil
	}
	for _, mapping := range strings.Split(masters, ",") {
		name, location, ok := strings.Cut(mapping, "=")
		if !ok || strings.TrimSpace(name) == "" || strings.TrimSpace(location) == "" {
			return nil, fmt.Errorf("invalid --master-repo mapping %q (want name=path)", mapping)
		}
		policy.ExplicitRepos[strings.TrimSpace(name)] = strings.TrimSpace(location)
	}
	return policy, nil
}

type cacheEbuild struct {
	Category string
	Package  string
	Version  g2.VersionData
}

// discoverCacheEbuilds is deliberately stricter than the site generator's
// repository parser. Cache verification and cleanup must have a complete
// inventory before they can claim success or delete an apparent orphan.
func discoverCacheEbuilds(cfs g2.CacheFS, repoDir string) ([]cacheEbuild, error) {
	categoryNames := make([]string, 0)
	categoriesPath := filepath.ToSlash(filepath.Join(repoDir, "profiles", "categories"))
	contents, err := fs.ReadFile(cfs, categoriesPath)
	if err == nil {
		for _, line := range strings.Split(string(contents), "\n") {
			name := strings.TrimSpace(line)
			if name != "" && !strings.HasPrefix(name, "#") {
				categoryNames = append(categoryNames, name)
			}
		}
	} else if os.IsNotExist(err) {
		entries, rootErr := fs.ReadDir(cfs, repoDir)
		if rootErr != nil {
			return nil, fmt.Errorf("reading repository root %s: %w", repoDir, rootErr)
		}
		for _, entry := range entries {
			if entry.IsDir() && !isIgnoredDir(entry.Name()) {
				categoryNames = append(categoryNames, entry.Name())
			}
		}
	} else {
		return nil, fmt.Errorf("reading categories file %s: %w", categoriesPath, err)
	}

	result := make([]cacheEbuild, 0)
	for _, category := range categoryNames {
		categoryPath := filepath.ToSlash(filepath.Join(repoDir, category))
		packages, err := fs.ReadDir(cfs, categoryPath)
		if err != nil {
			return nil, fmt.Errorf("reading category directory %s: %w", categoryPath, err)
		}
		for _, pkg := range packages {
			if !pkg.IsDir() || strings.HasPrefix(pkg.Name(), ".") {
				continue
			}
			packagePath := filepath.ToSlash(filepath.Join(categoryPath, pkg.Name()))
			files, err := fs.ReadDir(cfs, packagePath)
			if err != nil {
				return nil, fmt.Errorf("reading package directory %s: %w", packagePath, err)
			}
			for _, file := range files {
				if file.IsDir() || !strings.HasSuffix(file.Name(), ".ebuild") {
					continue
				}
				ebuildPath := filepath.ToSlash(filepath.Join(packagePath, file.Name()))
				ebuild, err := g2.ParseEbuild(cfs, ebuildPath, g2.ParseFull)
				if err != nil {
					return nil, fmt.Errorf("parsing ebuild %s: %w", ebuildPath, err)
				}
				pvr := g2.ParseEbuildVariables(file.Name())["PVR"]
				if pvr == "" {
					return nil, fmt.Errorf("deriving PVR from ebuild name %s", ebuildPath)
				}
				result = append(result, cacheEbuild{Category: category, Package: pkg.Name(), Version: g2.VersionData{Version: ebuild.Vars["PV"], PVR: pvr, Ebuild: ebuild}})
			}
		}
	}
	return result, nil
}

func doCacheReconcile(cfs g2.CacheFS, repoDir string, policy *g2.CachePolicy) error {
	log.Printf("Starting cache reconciliation...")

	// 1. generate/update expected cache entries
	log.Printf("Generating expected cache entries...")
	if err := g2.GenerateCacheFS(cfs, repoDir, nil, policy); err != nil {
		return fmt.Errorf("failed generating cache: %w", err)
	}

	// 2. & 3. clean up orphans and handle legacy paths
	log.Printf("Cleaning up stale cache entries and legacy paths...")
	if err := doCacheClean(cfs, repoDir); err != nil {
		return fmt.Errorf("failed cleaning cache: %w", err)
	}

	// 4. verify resulting repository state
	log.Printf("Verifying resulting cache consistency...")
	if err := doCacheVerify(cfs, repoDir, policy); err != nil {
		return fmt.Errorf("cache consistency check failed after reconciliation: %w", err)
	}

	fmt.Println("Cache reconciliation completed successfully.")
	return nil
}
