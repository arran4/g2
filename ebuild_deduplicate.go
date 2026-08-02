package g2

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ebuildItem struct {
	path    string
	version string
	digest  string
	grade   string
	slot    string
}

// DeduplicateEbuilds scans the given targets (directories or files) for ebuilds,
// groups them by SLOT and version grade, removes duplicates based on content digest
// (ignoring `# Generated via:` lines), keeps the highest version within each grade,
// and cleans up empty package directories (including `Manifest`, `metadata.xml`,
// and `md5-cache` entries). It returns a slice of file paths that were removed.
func DeduplicateEbuilds(targets []string) ([]string, error) {
	if len(targets) == 0 {
		targets = []string{"."}
	}

	// We collect all ebuild files first
	var ebuildFiles []string
	for _, target := range targets {
		info, err := os.Stat(target)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", target, err)
		}
		if !info.IsDir() {
			if strings.HasSuffix(target, ".ebuild") {
				ebuildFiles = append(ebuildFiles, target)
			}
			continue
		}

		err = filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".ebuild") {
				ebuildFiles = append(ebuildFiles, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking %s: %w", target, err)
		}
	}

	packages := make(map[string]map[string]map[string][]ebuildItem) // pkgDir -> slot -> grade -> items

	for _, ebuildPath := range ebuildFiles {
		pkgDir := filepath.Dir(ebuildPath)
		base := filepath.Base(ebuildPath)

		vars := ParseEbuildVariables(base)
		if vars == nil {
			continue // Could not parse P/PN/PV
		}
		version := vars["PV"]

		contentBytes, err := os.ReadFile(ebuildPath)
		if err != nil {
			log.Printf("Warning: failed to read %s: %v", ebuildPath, err)
			continue
		}

		lines := strings.Split(string(contentBytes), "\n")
		var hashContent []string
		slot := "0"
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "# Generated via:") {
				continue
			}
			hashContent = append(hashContent, line)

			if strings.HasPrefix(trimmed, "SLOT=") {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 {
					slot = strings.Trim(parts[1], `"'`)
				}
			}
		}

		hasher := md5.New()
		hasher.Write([]byte(strings.Join(hashContent, "\n")))
		digest := hex.EncodeToString(hasher.Sum(nil))

		grade := "release"
		vLower := strings.ToLower(version)
		if strings.Contains(vLower, "9999") {
			grade = "9999"
		} else {
			for _, tag := range []string{"alpha", "beta", "rc", "pre", "test"} {
				if strings.Contains(vLower, tag) {
					grade = tag
					break
				}
			}
		}

		item := ebuildItem{
			path:    ebuildPath,
			version: version,
			digest:  digest,
			grade:   grade,
			slot:    slot,
		}

		if packages[pkgDir] == nil {
			packages[pkgDir] = make(map[string]map[string][]ebuildItem)
		}
		if packages[pkgDir][slot] == nil {
			packages[pkgDir][slot] = make(map[string][]ebuildItem)
		}
		packages[pkgDir][slot][grade] = append(packages[pkgDir][slot][grade], item)
	}

	var removedFiles []string

	for pkgDir, slots := range packages {
		for _, grades := range slots {
			for _, items := range grades {
				// Deduplicate by digest first
				digestGroups := make(map[string][]ebuildItem)
				for _, item := range items {
					digestGroups[item.digest] = append(digestGroups[item.digest], item)
				}

				var keptItems []ebuildItem
				for _, dgItems := range digestGroups {
					if len(dgItems) > 1 {
						sort.Slice(dgItems, func(i, j int) bool {
							return CompareVersions(dgItems[i].version, dgItems[j].version) < 0
						})
						for i := 0; i < len(dgItems)-1; i++ {
							if err := os.Remove(dgItems[i].path); err == nil {
								removedFiles = append(removedFiles, dgItems[i].path)
							} else {
								log.Printf("Failed to remove duplicate %s: %v", dgItems[i].path, err)
							}
						}
					}
					keptItems = append(keptItems, dgItems[len(dgItems)-1])
				}

				// Keep only highest version in grade/slot
				if len(keptItems) > 1 {
					sort.Slice(keptItems, func(i, j int) bool {
						return CompareVersions(keptItems[i].version, keptItems[j].version) < 0
					})
					for i := 0; i < len(keptItems)-1; i++ {
						if err := os.Remove(keptItems[i].path); err == nil {
							removedFiles = append(removedFiles, keptItems[i].path)
						} else {
							log.Printf("Failed to remove older version %s: %v", keptItems[i].path, err)
						}
					}
				}
			}
		}

		// Cleanup after modifying a package
		removedSomething := false
		for _, rem := range removedFiles {
			if filepath.Clean(filepath.Dir(rem)) == filepath.Clean(pkgDir) {
				removedSomething = true
				break
			}
		}

		if removedSomething {
			entries, _ := os.ReadDir(pkgDir)
			ebuildCount := 0
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".ebuild") {
					ebuildCount++
				}
			}

			manifestPath := filepath.Join(pkgDir, "Manifest")
			if ebuildCount > 0 {
				if manifest, err := ParseManifest(manifestPath); err == nil {
					_ = CleanManifest(os.DirFS(pkgDir), ".", manifest)
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

					// repo root would be len(parts)-2 levels up
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
	}

	return removedFiles, nil
}
