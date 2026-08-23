package md5cache

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/arran4/g2"
)

func createSyntheticOverlay(b *testing.B, tempDir string) ([]*g2.PackageData, string) {
	b.Helper()

	eclassDir := filepath.Join(tempDir, "eclass")
	_ = os.MkdirAll(eclassDir, 0755)

	var eclassHashes []string
	for i := 0; i < 10; i++ {
		content := []byte(fmt.Sprintf("eclass content %d", i))
		eclassFile := filepath.Join(eclassDir, fmt.Sprintf("eclass%d.eclass", i))
		_ = os.WriteFile(eclassFile, content, 0644)
		eclassHashes = append(eclassHashes, fmt.Sprintf("%x", md5.Sum(content)))
	}

	var packages []*g2.PackageData
	for p := 0; p < 100; p++ {
		pkgName := fmt.Sprintf("pkg%d", p)
		pkg := &g2.PackageData{
			Category: "app-test",
			Name:     pkgName,
			Versions: make([]g2.VersionData, 2),
		}

		cacheDir := filepath.Join(tempDir, "metadata", "md5-cache", "app-test")
		_ = os.MkdirAll(cacheDir, 0755)

		ebuildDir := filepath.Join(tempDir, "app-test", pkg.Name)
		_ = os.MkdirAll(ebuildDir, 0755)

		for v := 0; v < 2; v++ {
			ver := fmt.Sprintf("%d.0", v)
			pkg.Versions[v] = g2.VersionData{
				Version: ver,
				Ebuild:  &g2.Ebuild{},
			}

			cacheFile := filepath.Join(cacheDir, fmt.Sprintf("%s-%s", pkgName, ver))
			ebuildFile := filepath.Join(ebuildDir, fmt.Sprintf("%s-%s.ebuild", pkgName, ver))

			ebuildContent := []byte("EAPI=8\n")
			_ = os.WriteFile(ebuildFile, ebuildContent, 0644)
			ebuildHash := fmt.Sprintf("%x", md5.Sum(ebuildContent))

			eclassesLine := ""
			for i := 0; i < 10; i++ {
				if i > 0 {
					eclassesLine += "\t"
				}
				eclassesLine += fmt.Sprintf("eclass%d\t%s", i, eclassHashes[i])
			}

			_ = os.WriteFile(cacheFile, []byte(fmt.Sprintf("_md5_=%s\n_eclasses_=%s\n", ebuildHash, eclassesLine)), 0644)
		}
		packages = append(packages, pkg)
	}
	return packages, eclassDir
}

func BenchmarkMD5CacheInvalid_ColdWholeOverlay(b *testing.B) {
	tempDir := b.TempDir()
	packages, _ := createSyntheticOverlay(b, tempDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Fresh rule with cold cache
		rule := &MD5CacheInvalidLintRule{}
		b.StartTimer()

		for _, pkg := range packages {
			rule.Lint(tempDir, pkg)
		}
	}
}

func BenchmarkMD5CacheInvalid_WarmWholeOverlay(b *testing.B) {
	tempDir := b.TempDir()
	packages, _ := createSyntheticOverlay(b, tempDir)

	rule := &MD5CacheInvalidLintRule{}
	// Warm up
	for _, pkg := range packages {
		rule.Lint(tempDir, pkg)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, pkg := range packages {
			rule.Lint(tempDir, pkg)
		}
	}
}

func BenchmarkMD5CacheInvalid_InvalidatedEclass(b *testing.B) {
	tempDir := b.TempDir()
	packages, eclassDir := createSyntheticOverlay(b, tempDir)

	rule := &MD5CacheInvalidLintRule{}
	for _, pkg := range packages {
		rule.Lint(tempDir, pkg)
	}

	eclassFile := filepath.Join(eclassDir, "eclass0.eclass")
	cacheDir := filepath.Join(tempDir, "metadata", "md5-cache", "app-test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		oldContent, _ := os.ReadFile(eclassFile)
		oldHash := fmt.Sprintf("%x", md5.Sum(oldContent))

		newContent := []byte(fmt.Sprintf("changed %d", i))
		newHash := fmt.Sprintf("%x", md5.Sum(newContent))
		_ = os.WriteFile(eclassFile, newContent, 0644)

		for _, pkg := range packages {
			for v := 0; v < 2; v++ {
				ver := fmt.Sprintf("%d.0", v)
				cacheFile := filepath.Join(cacheDir, fmt.Sprintf("%s-%s", pkg.Name, ver))
				data, _ := os.ReadFile(cacheFile)

				oldStr := []byte(fmt.Sprintf("eclass0\t%s", oldHash))
				newStr := []byte(fmt.Sprintf("eclass0\t%s", newHash))
				data = bytes.Replace(data, oldStr, newStr, 1)
				_ = os.WriteFile(cacheFile, data, 0644)
			}
		}

		b.StartTimer()
		for _, pkg := range packages {
			rule.Lint(tempDir, pkg)
		}
	}
}
