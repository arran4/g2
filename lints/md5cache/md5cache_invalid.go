package md5cache

import (
	"bufio"
	"container/list"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

/*
Note: The format of the md5-cache file was verified using the egencache tool in a gentoo/stage3 Docker container:
```bash
docker run --rm gentoo/stage3:latest bash -c "
  emerge --info
  mkdir -p /var/db/repos/testrepo/profiles
  mkdir -p /var/db/repos/testrepo/app-test/test
  echo 'testrepo' > /var/db/repos/testrepo/profiles/repo_name
  echo 'app-test' > /var/db/repos/testrepo/profiles/categories
  echo 'EAPI=8' > /var/db/repos/testrepo/app-test/test/test-1.0.ebuild
  echo 'DESCRIPTION=\"test\"' >> /var/db/repos/testrepo/app-test/test/test-1.0.ebuild
  echo 'SLOT=\"0\"' >> /var/db/repos/testrepo/app-test/test/test-1.0.ebuild

  mkdir -p /etc/portage/repos.conf
  cat << INN > /etc/portage/repos.conf/testrepo.conf
[testrepo]
location = /var/db/repos/testrepo
masters =
auto-sync = no
INN

  egencache --repo testrepo --update

  cat /var/db/repos/testrepo/metadata/md5-cache/app-test/test-1.0
"
```
The output format consists of `KEY=VALUE` pairs separated by newlines.
The `_md5_` key contains the md5sum of the ebuild.
The `_eclasses_` key contains pairs of `eclass_name eclass_md5` separated by whitespace (tab).
*/

const defaultEclassMD5CacheLimit = 5000

var ruleMD5CacheInvalid = lints.RuleMetadata{
	ID:          "Md5CacheInvalid",
	Title:       "Invalid MD5 Cache",
	Description: "Verifies that md5-cache entries are correctly formatted, contain correct ebuild md5sums, and valid eclass md5sums.",
	References: []lints.RuleReference{
		{URL: "https://devmanual.gentoo.org/general-concepts/portage-cache/index.html", Label: "Gentoo Devmanual"},
	},
	Severity: lints.SeverityWarning,
	Source:   lints.SourceG2,
	Tags:     []string{"md5-cache", "site-quality"},
}

func init() {
	lints.RegisterRuleMetadata(ruleMD5CacheInvalid)
	lints.RegisterLintRule(&MD5CacheInvalidLintRule{})
}

type cacheEntry struct {
	path string
	md5  string
	info os.FileInfo
}

type MD5CacheInvalidLintRule struct {
	mu    sync.Mutex
	cache map[string]*list.Element
	lru   *list.List
	limit int
}

func canonicalEclassPath(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return absPath
	}
	return resolvedPath
}

func (r *MD5CacheInvalidLintRule) ensureCacheLocked() {
	if r.cache == nil {
		r.cache = make(map[string]*list.Element)
		r.lru = list.New()
		if r.limit == 0 {
			r.limit = defaultEclassMD5CacheLimit
		}
	}
}

func (r *MD5CacheInvalidLintRule) getEclassMD5(path string) (string, error) {
	canonicalPath := canonicalEclassPath(path)

	stat, err := os.Stat(canonicalPath)
	if err != nil {
		return "", err
	}

	r.mu.Lock()
	r.ensureCacheLocked()
	if elem, ok := r.cache[canonicalPath]; ok {
		entry := elem.Value.(*cacheEntry)
		// Validate using Size, ModTime, and os.SameFile
		if entry.info != nil && entry.info.Size() == stat.Size() && entry.info.ModTime().Equal(stat.ModTime()) && os.SameFile(entry.info, stat) {
			r.lru.MoveToFront(elem)
			r.mu.Unlock()
			return entry.md5, nil
		}
		// Invalidated, remove old entry
		r.lru.Remove(elem)
		delete(r.cache, canonicalPath)
	}
	r.mu.Unlock()

	// Hash outside lock to allow concurrency.
	var hash string
	var finalInfo os.FileInfo

	// Bounded retry if file mutates during read
	success := false
	for i := 0; i < 3; i++ {
		f, err := os.Open(canonicalPath)
		if err != nil {
			return "", err
		}

		before, err := f.Stat()
		if err != nil {
			_ = f.Close()
			return "", err
		}

		data, err := io.ReadAll(f)
		if err != nil {
			_ = f.Close()
			return "", err
		}

		after, err := f.Stat()
		_ = f.Close()

		if err != nil {
			return "", err
		}

		if before.Size() == after.Size() && before.ModTime().Equal(after.ModTime()) && os.SameFile(before, after) {
			hash = fmt.Sprintf("%x", md5.Sum(data))
			finalInfo = after
			success = true
			break
		}
	}

	if !success {
		return "", fmt.Errorf("file mutated concurrently during read: %s", canonicalPath)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Ensure initialized in case it was nil before
	r.ensureCacheLocked()

	// Check again in case another goroutine did it
	if elem, ok := r.cache[canonicalPath]; ok {
		entry := elem.Value.(*cacheEntry)
		if entry.info != nil && entry.info.Size() == finalInfo.Size() && entry.info.ModTime().Equal(finalInfo.ModTime()) && os.SameFile(entry.info, finalInfo) {
			r.lru.MoveToFront(elem)
			return entry.md5, nil
		}
		r.lru.Remove(elem)
		delete(r.cache, canonicalPath)
	}

	if r.limit > 0 && r.lru.Len() >= r.limit {
		back := r.lru.Back()
		if back != nil {
			oldEntry := back.Value.(*cacheEntry)
			delete(r.cache, oldEntry.path)
			r.lru.Remove(back)
		}
	}

	newEntry := &cacheEntry{
		path: canonicalPath,
		md5:  hash,
		info: finalInfo,
	}
	elem := r.lru.PushFront(newEntry)
	r.cache[canonicalPath] = elem

	return hash, nil
}

func (r *MD5CacheInvalidLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *MD5CacheInvalidLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityWarning

	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["Md5CacheInvalid"]; ok {
			if val == "ignore" {
				return nil
			}
			if val == "notice" || val == "error" || val == "warning" {
				switch val {
				case "notice":
					severity = lints.SeverityNotice
				case "error":
					severity = lints.SeverityError
				case "warning":
					severity = lints.SeverityWarning
				}
			}
		}
	}

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil {
			cachePath := filepath.Join(repoDir, "metadata", "md5-cache", pkg.Category, pkg.Name+"-"+ver.Version)

			f, err := os.Open(cachePath)
			if err != nil {
				// Handled by missing md5-cache rule
				continue
			}

			scanner := bufio.NewScanner(f)
			hasEbuildMd5 := false
			var ebuildMd5 string
			var eclassesLine string
			for scanner.Scan() {
				line := scanner.Text()
				parts := strings.SplitN(line, "=", 2)
				if len(parts) != 2 {
					sevStr := string(severity)
					sevTitle := strings.ToUpper(sevStr[:1]) + sevStr[1:]
					res := lints.LintResult{
						RuleMetadata: ruleMD5CacheInvalid,
						Message:      fmt.Sprintf("[%s] Invalid format in md5-cache for %s-%s: %s", sevTitle, pkg.Name, ver.Version, line),
						Package:      pkg.Category + "/" + pkg.Name,
					}
					res.RuleMetadata.Severity = severity
					results = append(results, res)
					continue
				}

				switch parts[0] {
				case "_md5_":
					hasEbuildMd5 = true
					ebuildMd5 = parts[1]
				case "_eclasses_":
					eclassesLine = parts[1]
				}
			}
			_ = f.Close()

			sevStr := string(severity)
			sevTitle := strings.ToUpper(sevStr[:1]) + sevStr[1:]

			if !hasEbuildMd5 {
				res := lints.LintResult{
					RuleMetadata: ruleMD5CacheInvalid,
					Message:      fmt.Sprintf("[%s] Missing _md5_ in md5-cache for %s-%s", sevTitle, pkg.Name, ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			} else {
				ebuildPath := filepath.Join(repoDir, pkg.Category, pkg.Name, pkg.Name+"-"+ver.Version+".ebuild")
				ebuildData, err := os.ReadFile(ebuildPath)
				if err == nil {
					actualMd5 := fmt.Sprintf("%x", md5.Sum(ebuildData))
					if actualMd5 != ebuildMd5 {
						res := lints.LintResult{
							RuleMetadata: ruleMD5CacheInvalid,
							Message:      fmt.Sprintf("[%s] Incorrect _md5_ in md5-cache for %s-%s. Expected %s, got %s", sevTitle, pkg.Name, ver.Version, actualMd5, ebuildMd5),
							Package:      pkg.Category + "/" + pkg.Name,
						}
						res.RuleMetadata.Severity = severity
						results = append(results, res)
					}
				}
			}

			if eclassesLine != "" {
				eclassParts := strings.Fields(eclassesLine)
				if len(eclassParts)%2 != 0 {
					res := lints.LintResult{
						RuleMetadata: ruleMD5CacheInvalid,
						Message:      fmt.Sprintf("[%s] Invalid _eclasses_ format in md5-cache for %s-%s", sevTitle, pkg.Name, ver.Version),
						Package:      pkg.Category + "/" + pkg.Name,
					}
					res.RuleMetadata.Severity = severity
					results = append(results, res)
				} else {
					for i := 0; i < len(eclassParts); i += 2 {
						eclassName := eclassParts[i]
						eclassMd5 := eclassParts[i+1]

						eclassPath := filepath.Join(repoDir, "eclass", eclassName+".eclass")
						actualEclassMd5, err := r.getEclassMD5(eclassPath)

						if err == nil && actualEclassMd5 != eclassMd5 {
							res := lints.LintResult{
								RuleMetadata: ruleMD5CacheInvalid,
								Message:      fmt.Sprintf("[%s] Incorrect eclass md5 for %s in md5-cache of %s-%s. Expected %s, got %s", sevTitle, eclassName, pkg.Name, ver.Version, actualEclassMd5, eclassMd5),
								Package:      pkg.Category + "/" + pkg.Name,
							}
							res.RuleMetadata.Severity = severity
							results = append(results, res)
						}
					}
				}
			}
		}
	}

	return results
}
