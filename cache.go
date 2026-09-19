package g2

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
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
func GenerateCache(repoDir string, targetPkgs []string, policy *CachePolicy) error {
	cfs := NewOsCacheFS(repoDir)
	return GenerateCacheFS(cfs, ".", targetPkgs, policy)
}

// GenerateCacheFS generates the cache for the repository using a custom CacheFS.
func GenerateCacheFS(cfs CacheFS, repoDir string, targetPkgs []string, policy *CachePolicy) error {
	if policy == nil {
		policy = NewCachePolicy(CacheModeCI)
	}
	if err := policy.Validate(); err != nil {
		return err
	}
	layoutConfPath := filepath.ToSlash(filepath.Join(repoDir, "metadata", "layout.conf"))
	var lc *LayoutConf
	if f, err := cfs.Open(layoutConfPath); err == nil {
		lc, err = ParseLayoutConfFromReader(f)
		if err != nil {
			_ = f.Close()
			return fmt.Errorf("parsing layout.conf %s: %w", layoutConfPath, err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("closing layout.conf %s: %w", layoutConfPath, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("opening layout.conf %s: %w", layoutConfPath, err)
	}

	cacheFormats := []string{"md5-dict"} // Default if not found
	if lc != nil {
		if lc.HasKey("cache-formats") {
			cacheFormats = lc.GetValuesAsSlice("cache-formats")
		}
	}

	eclassResolver, err := BuildEclassResolver(cfs, repoDir, policy)
	if err != nil {
		return fmt.Errorf("building eclass resolver: %w", err)
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
		} else if errors.Is(err, fs.ErrNotExist) {
			// fallback: scan directory for things that look like categories.
			entries, err := fs.ReadDir(cfs, repoDir)
			if err != nil {
				return fmt.Errorf("reading repository root %s while discovering categories: %w", repoDir, err)
			}
			for _, entry := range entries {
				if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") && entry.Name() != "metadata" && entry.Name() != "profiles" && entry.Name() != "eclass" {
					categories = append(categories, entry.Name())
				}
			}
		} else {
			return fmt.Errorf("reading categories from %s: %w", repoDir, err)
		}

		// Read packages in each category
		for _, cat := range categories {
			catPath := filepath.Join(repoDir, cat)
			pkgEntries, err := fs.ReadDir(cfs, filepath.ToSlash(catPath))
			if err != nil {
				return fmt.Errorf("reading category directory %s: %w", catPath, err)
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
					return fmt.Errorf("reading package directory %s: %w", pkgPath, err)
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
						if err != nil {
							return fmt.Errorf("parsing ebuild %s: %w", ebuildPath, err)
						}
						return fmt.Errorf("parsing ebuild %s: no metadata was produced", ebuildPath)
					}

					// Extract PVR
					vars := ParseEbuildVariables(ebuildName)
					pvr, ok := vars["PVR"]
					if !ok || pvr == "" {
						continue
					}

					verData := VersionData{
						Version: vars["PV"],
						PVR:     pvr,
						Ebuild:  ebuild,
					}
					ident := GetCacheIdentity(repoDir, format, cat, pkgName, verData)
					if ident.CachePath == "" {
						continue // unsupported format
					}
					cacheDir := filepath.ToSlash(filepath.Dir(ident.CachePath))
					if err := cfs.MkdirAll(cacheDir, 0755); err != nil {
						return fmt.Errorf("creating cache directory %s: %w", cacheDir, err)
					}

					verCachePath := ident.CachePath

					expectedContentStr, status, err := GetExpectedCacheContent(cfs, ebuildPath, ebuild, policy, eclassResolver)
					if err != nil {
						return fmt.Errorf("getting expected cache for %s: %w", ident.CachePath, err)
					}
					if status == CacheSkipped {
						return fmt.Errorf("cannot generate authoritative cache for %s: %s", ident.EbuildPath, strings.Join(eclassResolver.MissingMasters(), ", "))
					}

					existingContent, err := fs.ReadFile(cfs, verCachePath)
					if err == nil && string(existingContent) == expectedContentStr {
						continue // Idempotent skip
					}

					f, err := cfs.Create(verCachePath)
					if err != nil {
						return fmt.Errorf("creating cache file %s: %w", verCachePath, err)
					}
					if _, err := f.Write([]byte(expectedContentStr)); err != nil {
						_ = f.Close()
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
// The version parameter should be the package version with revision (PVR), e.g. "0-r1" or "1.2.3".
// Returns an empty string if the format is not explicitly supported.
func GetCachePath(repoDir string, format string, category string, name string, version string) string {
	dir := GetCacheDir(repoDir, format, category)
	if dir == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join(dir, fmt.Sprintf("%s-%s", name, version)))
}

// CacheIdentity represents the resolved identity of a package version for caching purposes.
type CacheIdentity struct {
	Category   string
	Package    string
	PVR        string
	CachePath  string
	EbuildPath string
}

// GetEbuildPath returns the path to the ebuild file for a VersionData.
func GetEbuildPath(repoDir, category, name string, ver VersionData) string {
	pvr := ver.GetPVR()
	ebuildFile := fmt.Sprintf("%s-%s.ebuild", name, pvr)
	if repoDir == "" || repoDir == "." {
		return filepath.ToSlash(filepath.Join(category, name, ebuildFile))
	}
	return filepath.ToSlash(filepath.Join(repoDir, category, name, ebuildFile))
}

// GetCacheIdentity returns the resolved CacheIdentity for a given package and version.
func GetCacheIdentity(repoDir, format, category, pkgName string, ver VersionData) CacheIdentity {
	pvr := ver.GetPVR()
	return CacheIdentity{
		Category:   category,
		Package:    pkgName,
		PVR:        pvr,
		CachePath:  GetCachePath(repoDir, format, category, pkgName, pvr),
		EbuildPath: GetEbuildPath(repoDir, category, pkgName, ver),
	}
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

// GetExpectedCacheContent returns the expected cache string for an ebuild, or a CacheResultStatus indicating why it couldn't.
func GetExpectedCacheContent(cfs CacheFS, ebuildPath string, ebuild *Ebuild, policy *CachePolicy, eclassResolver *EclassResolver) (string, CacheResultStatus, error) {
	if ebuild == nil || ebuild.Vars == nil {
		return "", CacheError, fmt.Errorf("ebuild %s has no parsed metadata", ebuildPath)
	}
	if ebuild.SrcUriUncertain {
		return "", CacheError, fmt.Errorf("ebuild %s contains control flow or unresolved values; canonical cache metadata requires Portage evaluation", ebuildPath)
	}
	for key := range ebuild.UncertainVars {
		if isCacheVariable(key) {
			return "", CacheError, fmt.Errorf("ebuild %s has unresolved %s; canonical cache metadata requires Portage evaluation", ebuildPath, key)
		}
	}
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
	if err != nil {
		// ebuild read failure => CacheError and operation failure
		return "", CacheError, fmt.Errorf("failed to read ebuild file %s: %w", ebuildPath, err)
	}

	md5sum := fmt.Sprintf("%x", md5.Sum(ebuildContent))
	fmt.Fprintf(&expectedContent, "_md5_=%s\n", md5sum)

	if inherited := ebuild.Vars["INHERITED"]; inherited != "" {
		eclassParts, err := eclassClosure(eclassResolver, strings.Fields(inherited), map[string]bool{}, map[string]bool{})
		if err != nil {
			if policy.Mode == CacheModeCI && len(eclassResolver.MissingMasters()) > 0 && strings.Contains(err.Error(), "unavailable masters") {
				return "", CacheSkipped, nil
			}
			return "", CacheError, fmt.Errorf("resolving required eclass metadata for %s: %w", ebuildPath, err)
		}
		fmt.Fprintf(&expectedContent, "_eclasses_=%s\n", strings.Join(eclassParts, "\t"))
	}

	// Constructing expected data is not verification of an on-disk cache entry.
	return expectedContent.String(), CacheDrift, nil
}

// CompareCacheEntry gives verification a structured result. Expected-content
// construction intentionally never claims CacheVerified; that status is only
// emitted after the on-disk cache entry has been compared successfully.
func CompareCacheEntry(cfs CacheFS, cachePath, expected string) CacheResult {
	content, err := fs.ReadFile(cfs, cachePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return CacheResult{Status: CacheDrift, Path: cachePath, Message: "cache entry is missing"}
		}
		return CacheResult{Status: CacheError, Path: cachePath, Message: "reading cache entry", Error: err}
	}
	if string(content) != expected {
		return CacheResult{Status: CacheDrift, Path: cachePath, Message: "cache entry differs from canonical metadata"}
	}
	return CacheResult{Status: CacheVerified, Path: cachePath, Message: "cache entry matches canonical metadata"}
}

// eclassClosure produces the complete, deterministic transitive inheritance
// closure. Eclass content is hashed exactly as resolved; cycles are harmless
// because Portage does not need duplicate _eclasses_ records.
func eclassClosure(resolver *EclassResolver, names []string, visiting, seen map[string]bool) ([]string, error) {
	parts := make(map[string]string)
	ordered := make([]string, 0)
	var visit func(string) error
	visit = func(name string) error {
		if seen[name] || visiting[name] {
			return nil
		}
		visiting[name] = true
		content, repo, err := resolver.resolve(name)
		if err != nil {
			return err
		}
		parsed, err := ParseEbuild(repo.FS, path.Join("eclass", name+".eclass"), ParseFull)
		if err != nil {
			return fmt.Errorf("parsing eclass %q from repository %q: %w", name, repo.Name, err)
		}
		if parsed.SrcUriUncertain {
			return fmt.Errorf("eclass %q from repository %q contains control flow or an unmodelled command; canonical cache metadata requires Portage evaluation", name, repo.Name)
		}
		for key := range parsed.UncertainVars {
			if isCacheVariable(key) {
				return fmt.Errorf("eclass %q from repository %q has unresolved %s; canonical cache metadata requires Portage evaluation", name, repo.Name, key)
			}
		}
		for key, value := range parsed.Vars {
			if key != "INHERITED" && value != "" && isCacheVariable(key) {
				return fmt.Errorf("eclass %q from repository %q contributes %s; canonical cache metadata requires Portage evaluation", name, repo.Name, key)
			}
		}
		for _, child := range strings.Fields(parsed.Vars["INHERITED"]) {
			if err := visit(child); err != nil {
				return err
			}
		}
		parts[name] = fmt.Sprintf("%s\t%x", name, md5.Sum(content))
		ordered = append(ordered, name)
		visiting[name] = false
		seen[name] = true
		return nil
	}
	for _, name := range names {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	result := make([]string, 0, len(ordered))
	for _, name := range ordered {
		result = append(result, parts[name])
	}
	return result, nil
}
