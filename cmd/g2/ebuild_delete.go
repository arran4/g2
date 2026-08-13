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

func (cfg *CmdEbuildArgConfig) cmdEbuildDelete(args []string) error {
	fset := flag.NewFlagSet("delete", flag.ExitOnError)
	repoDir := fset.String("repo", ".", "Repository directory (default: current directory)")

	if err := fset.Parse(args); err != nil {
		return err
	}

	if fset.NArg() == 0 {
		return fmt.Errorf("usage: g2 ebuild delete <ebuild file | ebuild name + version range> [--repo <name / dir>]")
	}

	target := fset.Arg(0)

	var removedFiles []string

	if strings.HasSuffix(target, ".ebuild") {
		// Specific file
		absPath, err := filepath.Abs(target)
		if err != nil {
			return fmt.Errorf("resolving target path: %w", err)
		}

		if err := os.Remove(absPath); err != nil {
			return fmt.Errorf("failed to remove %s: %w", absPath, err)
		}
		removedFiles = append(removedFiles, absPath)
	} else {
		// Package atom or name + version range
		atom := g2.ParsePackageAtom(target)

		repoPath := *repoDir
		if repoPath == "" {
			repoPath = "."
		}

		// Find matching ebuilds
		var ebuildsToRemove []string

		err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, ".ebuild") {
				vars := g2.ParseEbuildVariables(filepath.Base(path))
				if vars != nil {
					cat := ""
					// Extract category from path
					relPath, err := filepath.Rel(repoPath, path)
					if err == nil {
						parts := strings.Split(relPath, string(filepath.Separator))
						if len(parts) >= 3 {
							cat = parts[len(parts)-3]
						}
					}

					match := true
					if atom.Category != "" && cat != "" && atom.Category != cat {
						match = false
					}
					if atom.Name != "" && vars["PN"] != atom.Name {
						match = false
					}

					// Basic version matching if specified
					if atom.Version != "" {
						cmp := g2.CompareVersions(vars["PV"], atom.Version)

						switch atom.Operator {
						case "=":
							if cmp != 0 {
								match = false
							}
						case "<":
							if cmp >= 0 {
								match = false
							}
						case "<=":
							if cmp > 0 {
								match = false
							}
						case ">":
							if cmp <= 0 {
								match = false
							}
						case ">=":
							if cmp < 0 {
								match = false
							}
						case "~":
							// Roughly same base version
							pvParts := strings.Split(vars["PV"], ".")
							avParts := strings.Split(atom.Version, ".")
							if len(pvParts) > 0 && len(avParts) > 0 && pvParts[0] != avParts[0] {
								match = false
							}
						default:
							// If no operator is given but version is, default to exact match or prefix
							if vars["PV"] != atom.Version {
								match = false
							}
						}
					}

					if match {
						ebuildsToRemove = append(ebuildsToRemove, path)
					}
				}
			}
			return nil
		})

		if err != nil {
			return fmt.Errorf("walking repo: %w", err)
		}

		if len(ebuildsToRemove) == 0 {
			return fmt.Errorf("no matching ebuilds found for %s", target)
		}

		for _, path := range ebuildsToRemove {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove %s: %w", path, err)
			}
			removedFiles = append(removedFiles, path)
		}
	}

	repoPath := *repoDir
	if repoPath == "" {
		repoPath = "."
	}

	// Clean up for each removed file's directory
	pkgDirs := make(map[string]bool)
	for _, rem := range removedFiles {
		pkgDirs[filepath.Dir(rem)] = true
	}

	for pkgDir := range pkgDirs {
		entries, _ := os.ReadDir(pkgDir)
		ebuildCount := 0
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".ebuild") {
				ebuildCount++
			}
		}

		manifestPath := filepath.Join(pkgDir, "Manifest")
		if ebuildCount > 0 {
			if manifest, err := g2.ParseManifest(manifestPath); err == nil {
				_ = g2.CleanManifest(os.DirFS(pkgDir), ".", manifest)
				_ = os.WriteFile(manifestPath, []byte(manifest.String()), 0644)
			}
		} else {
			_ = os.Remove(manifestPath)
			_ = os.Remove(filepath.Join(pkgDir, "metadata.xml"))

			// Attempt to remove md5-cache
			parts := strings.Split(filepath.ToSlash(pkgDir), "/")
			if len(parts) >= 2 {
				category := parts[len(parts)-2]
				pkg := parts[len(parts)-1]

				var repoRoot string
				if filepath.IsAbs(pkgDir) {
					repoRoot = filepath.Join(parts[:len(parts)-2]...)
					repoRoot = "/" + repoRoot
				} else {
					repoRoot = filepath.Join(parts[:len(parts)-2]...)
				}

				md5CacheDir := filepath.Join(repoRoot, "metadata", "md5-cache", category)
				if cacheEntries, err := os.ReadDir(md5CacheDir); err == nil {
					for _, ce := range cacheEntries {
						if strings.HasPrefix(ce.Name(), pkg+"-") && len(ce.Name()) > len(pkg)+1 && (ce.Name()[len(pkg)] == '-' && ce.Name()[len(pkg)+1] >= '0' && ce.Name()[len(pkg)+1] <= '9') {
							_ = os.Remove(filepath.Join(md5CacheDir, ce.Name()))
						}
					}
				}
			}

			_ = os.RemoveAll(pkgDir)
		}
	}

	// Re-run manifest check for all modified packages
	manifestCfg := &CmdManifestArgConfig{MainArgConfig: cfg.MainArgConfig}
	for pkgDir := range pkgDirs {
		if _, err := os.Stat(pkgDir); err == nil {
			if err := manifestCfg.cmdVerify([]string{"-fix", pkgDir}, g2.AllHashes); err != nil {
				log.Printf("Warning: updating manifest: %v", err)
			}
		}
	}

	if err := cfg.cmdUseDiscover([]string{repoPath}); err != nil {
		log.Printf("Warning: updating use.desc/use.local.desc: %v", err)
	}

	if err := cfg.cmdPkgDescIndexGenerate([]string{repoPath}); err != nil {
		log.Printf("Warning: generating pkg_desc_index: %v", err)
	}

	if err := cfg.cmdCacheGenerate([]string{repoPath}); err != nil {
		log.Printf("Warning: generating md5-cache: %v", err)
	}

	fmt.Printf("Deleted %d ebuild(s) and cleaned up associated files.\n", len(removedFiles))
	return nil
}
