package md5cache

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/arran4/g2"
)

func BenchmarkMD5CacheInvalid(b *testing.B) {
	rule := &MD5CacheInvalidLintRule{}
	// Force initialization if not set
	_, _ = rule.getEclassMD5("")

	tempDir, err := os.MkdirTemp("", "bench")
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	eclassDir := filepath.Join(tempDir, "eclass")
	_ = os.MkdirAll(eclassDir, 0755)

	// Create 10 different eclasses
	for i := 0; i < 10; i++ {
		eclassContent := fmt.Sprintf("test content %d\n", i)
		eclassFile := filepath.Join(eclassDir, fmt.Sprintf("eclass%d.eclass", i))
		_ = os.WriteFile(eclassFile, []byte(eclassContent), 0644)
	}

	// Create 100 packages, each with 2 versions, referencing all 10 eclasses
	var packages []*g2.PackageData
	for p := 0; p < 100; p++ {
		pkg := &g2.PackageData{
			Category: "app-test",
			Name:     fmt.Sprintf("pkg%d", p),
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

			cacheFile := filepath.Join(cacheDir, fmt.Sprintf("%s-%s", pkg.Name, ver))
			ebuildFile := filepath.Join(ebuildDir, fmt.Sprintf("%s-%s.ebuild", pkg.Name, ver))
			_ = os.WriteFile(ebuildFile, []byte("EAPI=8\n"), 0644)

			// eclassesLine format
			eclassesLine := ""
			for i := 0; i < 10; i++ {
				eclassesLine += fmt.Sprintf("eclass%d\t1234567890abcdef\t", i)
			}

			_ = os.WriteFile(cacheFile, []byte(fmt.Sprintf("_md5_=1234567890abcdef\n_eclasses_=%s\n", eclassesLine)), 0644)
		}
		packages = append(packages, pkg)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, pkg := range packages {
			rule.Lint(tempDir, pkg)
		}
	}
}
