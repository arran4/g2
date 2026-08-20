package md5cache

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/arran4/g2"
)

func TestMD5CacheInvalidLintRule(t *testing.T) {
	rule := &MD5CacheInvalidLintRule{}

	tempDir := t.TempDir()

	pkg := &g2.PackageData{
		Category: "app-misc",
		Name:     "foo",
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				Ebuild:  &g2.Ebuild{},
			},
		},
	}

	// 1. Valid cache
	cacheDir := filepath.Join(tempDir, "metadata", "md5-cache", "app-misc")
	_ = os.MkdirAll(cacheDir, 0755)
	cacheFile := filepath.Join(cacheDir, "foo-1.0")

	ebuildDir := filepath.Join(tempDir, "app-misc", "foo")
	_ = os.MkdirAll(ebuildDir, 0755)
	ebuildFile := filepath.Join(ebuildDir, "foo-1.0.ebuild")
	_ = os.WriteFile(ebuildFile, []byte("EAPI=8\n"), 0644)
	md5sum := fmt.Sprintf("%x", md5.Sum([]byte("EAPI=8\n")))

	eclassDir := filepath.Join(tempDir, "eclass")
	_ = os.MkdirAll(eclassDir, 0755)
	eclassFile := filepath.Join(eclassDir, "test.eclass")
	_ = os.WriteFile(eclassFile, []byte("# test\n"), 0644)
	eclassMd5 := fmt.Sprintf("%x", md5.Sum([]byte("# test\n")))

	_ = os.WriteFile(cacheFile, []byte(fmt.Sprintf("_md5_=%s\n_eclasses_=test\t%s\n", md5sum, eclassMd5)), 0644)

	results := rule.Lint(tempDir, pkg, nil)
	if len(results) != 0 {
		t.Errorf("Expected 0 results for valid cache, got %d: %v", len(results), results)
	}

	// 2. Invalid ebuild md5
	_ = os.WriteFile(cacheFile, []byte(fmt.Sprintf("_md5_=%s\n_eclasses_=test\t%s\n", "invalidmd5", eclassMd5)), 0644)
	results = rule.Lint(tempDir, pkg, nil)
	if len(results) != 1 {
		t.Errorf("Expected 1 result for invalid ebuild md5, got %d", len(results))
	} else if results[0].Message != fmt.Sprintf("[Warning] Incorrect _md5_ in md5-cache for foo-1.0. Expected %s, got invalidmd5", md5sum) {
		t.Errorf("Unexpected message: %s", results[0].Message)
	}

	// 3. Invalid eclass md5
	_ = os.WriteFile(cacheFile, []byte(fmt.Sprintf("_md5_=%s\n_eclasses_=test\t%s\n", md5sum, "invalidmd5")), 0644)
	results = rule.Lint(tempDir, pkg, nil)
	if len(results) != 1 {
		t.Errorf("Expected 1 result for invalid eclass md5, got %d", len(results))
	} else if results[0].Message != fmt.Sprintf("[Warning] Incorrect eclass md5 for test in md5-cache of foo-1.0. Expected %s, got invalidmd5", eclassMd5) {
		t.Errorf("Unexpected message: %s", results[0].Message)
	}

	// 4. Missing _md5_
	_ = os.WriteFile(cacheFile, []byte(fmt.Sprintf("_eclasses_=test\t%s\n", eclassMd5)), 0644)
	results = rule.Lint(tempDir, pkg, nil)
	if len(results) != 1 {
		t.Errorf("Expected 1 result for missing _md5_, got %d", len(results))
	} else if results[0].Message != "[Warning] Missing _md5_ in md5-cache for foo-1.0" {
		t.Errorf("Unexpected message: %s", results[0].Message)
	}

	// 5. Invalid format
	_ = os.WriteFile(cacheFile, []byte(fmt.Sprintf("_md5_=%s\ninvalidline\n", md5sum)), 0644)
	results = rule.Lint(tempDir, pkg, nil)
	if len(results) != 1 {
		t.Errorf("Expected 1 result for invalid format, got %d", len(results))
	} else if results[0].Message != "[Warning] Invalid format in md5-cache for foo-1.0: invalidline" {
		t.Errorf("Unexpected message: %s", results[0].Message)
	}
}
