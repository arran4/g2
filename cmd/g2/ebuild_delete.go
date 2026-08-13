package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
)

func (cfg *CmdEbuildArgConfig) cmdEbuildDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	repoFlag := fs.String("repo", ".", "Repo name or dir")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	targets := fs.Args()
	if len(targets) == 0 {
		return fmt.Errorf("usage: g2 ebuild delete <ebuild file | ebuild name + version range --repo <name / dir>>")
	}

	repoDir := *repoFlag
	if !filepath.IsAbs(repoDir) && repoDir != "." {
		if abs, err := filepath.Abs(repoDir); err == nil {
			repoDir = abs
		}
	}

	var filesToRemove []string

	for _, target := range targets {
		if strings.HasSuffix(target, ".ebuild") {
			// It's a file
			absTarget, err := filepath.Abs(target)
			if err != nil {
				return fmt.Errorf("resolving path %s: %w", target, err)
			}
			filesToRemove = append(filesToRemove, absTarget)
		} else {
			// Try parsing as package atom
			atom := g2.ParsePackageAtom(target)

			if atom.Category == "" || atom.Name == "" {
				return fmt.Errorf("invalid package atom: %s, category and name required", target)
			}

			pkgDir := filepath.Join(repoDir, atom.Category, atom.Name)
			entries, err := os.ReadDir(pkgDir)
			if err != nil {
				if os.IsNotExist(err) {
					log.Printf("Warning: package directory %s not found", pkgDir)
					continue
				}
				return fmt.Errorf("reading package dir %s: %w", pkgDir, err)
			}

			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".ebuild") {
					continue
				}
				vars := g2.ParseEbuildVariables(e.Name())
				if vars == nil {
					continue
				}
				ebuildVer := vars["PV"]
				if atom.Version != "" {
					if atom.Version != ebuildVer {
						continue // Skip if versions don't match
					}
				}

				filesToRemove = append(filesToRemove, filepath.Join(pkgDir, e.Name()))
			}
		}
	}

	pkgDirsToClean := make(map[string]bool)

	for _, f := range filesToRemove {
		if err := os.Remove(f); err != nil {
			log.Printf("Failed to remove ebuild %s: %v", f, err)
			continue
		}
		log.Printf("Removed %s", f)
		pkgDirsToClean[filepath.Clean(filepath.Dir(f))] = true
	}

	for pkgDir := range pkgDirsToClean {
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

	return nil
}
