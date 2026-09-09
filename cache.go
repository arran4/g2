package g2

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CacheFS interface provides read and write abstraction for testability
type CacheFS interface {
	fs.FS
	MkdirAll(path string, perm os.FileMode) error
	Create(name string) (io.WriteCloser, error)
	Remove(name string) error
	RemoveAll(name string) error
	Walk(root string, fn fs.WalkDirFunc) error
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

func (o *OsCacheFS) RemoveAll(name string) error {
	return os.RemoveAll(filepath.Join(o.base, name))
}

func (o *OsCacheFS) Remove(name string) error {
	return os.Remove(filepath.Join(o.base, name))
}

func (o *OsCacheFS) Walk(root string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(filepath.Join(o.base, root), func(path string, d fs.DirEntry, err error) error {
		relPath, _ := filepath.Rel(o.base, path)
		return fn(relPath, d, err)
	})
}

func (o *OsCacheFS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(filepath.Join(o.base, name))
}

// GenerateCache generates the cache for the repository.
func GenerateCache(repoDir string, targetPkgs []string, genEclasses bool) error {
	cfs := NewOsCacheFS(repoDir)
	return GenerateCacheFS(cfs, ".", targetPkgs, genEclasses)
}

// GenerateCacheFS generates the cache for the repository using a custom CacheFS.
func GenerateCacheFS(cfs CacheFS, repoDir string, targetPkgs []string, genEclasses bool) error {
	layoutConfPath := filepath.ToSlash(filepath.Join(repoDir, "metadata", "layout.conf"))
	var lc *LayoutConf
	if f, err := cfs.Open(layoutConfPath); err == nil {
		lc, err = ParseLayoutConfFromReader(f)
		if err != nil {
			log.Printf("Warning: failed to parse layout.conf: %v", err)
			lc = nil
		}
		_ = f.Close()
	}

	cacheFormats := []string{"md5-dict"} // Default if not found
	if lc != nil {
		if lc.HasKey("cache-formats") {
			cacheFormats = lc.GetValuesAsSlice("cache-formats")
		}
	}

	for _, format := range cacheFormats {
		if format != "md5-dict" {
			log.Printf("Warning: Cache format '%s' is not supported. Skipping. Only md5-dict is supported.", format)
			continue
		}

		log.Printf("Generating cache for format: %s", format)

		// Iterate through categories
		categoriesBytes, err := fs.ReadFile(cfs, filepath.ToSlash(filepath.Join(repoDir, "profiles", "categories")))
		var categories []string
		if err == nil {
			for _, line := range strings.Split(string(categoriesBytes), "\n") {
				cat := strings.TrimSpace(line)
				if cat != "" && !strings.HasPrefix(cat, "#") {
					categories = append(categories, cat)
				}
			}
		} else {
			// fallback: scan directory for things that look like categories.
			entries, err := fs.ReadDir(cfs, repoDir)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") && entry.Name() != "metadata" && entry.Name() != "profiles" && entry.Name() != "eclass" {
						categories = append(categories, entry.Name())
					}
				}
			}
		}

		// Read packages in each category
		for _, cat := range categories {
			catPath := filepath.Join(repoDir, cat)
			pkgEntries, err := fs.ReadDir(cfs, filepath.ToSlash(catPath))
			if err != nil {
				continue
			}

			for _, pkgEntry := range pkgEntries {
				if !pkgEntry.IsDir() {
					continue
				}
				pkgName := pkgEntry.Name()
				if strings.HasPrefix(pkgName, ".") {
					continue
				}

				if len(targetPkgs) > 0 {
					qualified := cat + "/" + pkgName
					found := false
					for _, target := range targetPkgs {
						if target == qualified || target == pkgName {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}

				pkgPath := filepath.Join(catPath, pkgName)
				ebuildEntries, err := fs.ReadDir(cfs, filepath.ToSlash(pkgPath))
				if err != nil {
					continue
				}

				for _, ebuildEntry := range ebuildEntries {
					if ebuildEntry.IsDir() || !strings.HasSuffix(ebuildEntry.Name(), ".ebuild") {
						continue
					}

					ebuildName := ebuildEntry.Name()
					ebuildPath := filepath.Join(pkgPath, ebuildName)

					// Parse the ebuild
					ebuild, err := ParseEbuild(cfs, filepath.ToSlash(ebuildPath), ParseFull)
					if err != nil || ebuild == nil || ebuild.Vars == nil {
						continue
					}

					// Extract PV
					vars := ParseEbuildVariables(ebuildName)
					pv, ok := vars["PVR"]
					if !ok || pv == "" {
						continue
					}

					cacheDir := GetCacheDir(repoDir, format, cat)
					if cacheDir == "" {
						continue // unsupported format
					}
					if err := cfs.MkdirAll(cacheDir, 0755); err != nil {
						return fmt.Errorf("creating cache directory %s: %w", cacheDir, err)
					}

					verCachePath := GetCachePath(repoDir, format, cat, pkgName, pv)

					var keys []string
					for k := range ebuild.Vars {
						keys = append(keys, k)
					}
					sort.Strings(keys)

					var expectedContent strings.Builder
					for _, k := range keys {
						v := ebuild.Vars[k]
						if v != "" {
							if isCacheVariable(k) {
								fmt.Fprintf(&expectedContent, "%s=%s\n", k, v)
							}
						}
					}

					ebuildContent, err := fs.ReadFile(cfs, filepath.ToSlash(ebuildPath))
					if err == nil {
						md5sum := fmt.Sprintf("%x", md5.Sum(ebuildContent))
						fmt.Fprintf(&expectedContent, "_md5_=%s\n", md5sum)
					}

					if genEclasses {
						if inherited, ok := ebuild.Vars["INHERITED"]; ok && inherited != "" {
							eclasses := strings.Fields(inherited)
							var eclassParts []string
							for _, ec := range eclasses {
								eclassPath := filepath.ToSlash(filepath.Join("eclass", ec+".eclass"))
								ecContent, err := fs.ReadFile(cfs, eclassPath)
								if err == nil {
									ecMd5 := fmt.Sprintf("%x", md5.Sum(ecContent))
									eclassParts = append(eclassParts, ec, ecMd5)
								}
							}
							if len(eclassParts) > 0 {
								fmt.Fprintf(&expectedContent, "_eclasses_=%s\n", strings.Join(eclassParts, "\t"))
							}
						}
					}

					existingContent, err := fs.ReadFile(cfs, verCachePath)
					if err == nil && string(existingContent) == expectedContent.String() {
						continue // Idempotent skip
					}

					f, err := cfs.Create(verCachePath)
					if err != nil {
						return fmt.Errorf("creating cache file %s: %w", verCachePath, err)
					}
					if _, err := f.Write([]byte(expectedContent.String())); err != nil {
						f.Close()
						return fmt.Errorf("writing cache file %s: %w", verCachePath, err)
					}
					if err := f.Close(); err != nil {
						return fmt.Errorf("closing cache file %s: %w", verCachePath, err)
					}
				}
			}
		}
	}

	return nil
}

// GetCacheDir returns the canonical directory for the given cache format and category.
// Returns an empty string if the format is not explicitly supported.
func GetCacheDir(repoDir string, format string, category string) string {
	root := GetCacheRoot(repoDir, format)
	if root == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join(root, category))
}

// GetCachePath returns the canonical file path for a specific package version's cache entry.
// Returns an empty string if the format is not explicitly supported.
func GetCachePath(repoDir string, format string, category string, name string, version string) string {
	dir := GetCacheDir(repoDir, format, category)
	if dir == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join(dir, fmt.Sprintf("%s-%s", name, version)))
}

// GetLegacyCacheDir returns the non-canonical legacy cache directory, if any.
func GetLegacyCacheDir(repoDir string, format string) string {
	if format == "md5-dict" {
		return filepath.ToSlash(filepath.Join(repoDir, "metadata", "md5-dict"))
	}
	return ""
}

// GetCacheRoot returns the root directory for a given cache format.
// Returns an empty string if the format is not explicitly supported.
func GetCacheRoot(repoDir string, format string) string {
	switch format {
	case "md5-dict":
		return filepath.ToSlash(filepath.Join(repoDir, "metadata", "md5-cache"))
	case "pms":
		return filepath.ToSlash(filepath.Join(repoDir, "metadata", "pms"))
	default:
		return ""
	}
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
		"_eclasses_":     true,
		"DEFINED_PHASES": true,
	}
	return validKeys[key]
}
