package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/md5cache"
)

func TestCacheIntegration_MultilineLiteral(t *testing.T) {
	// Setup repo
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, "metadata"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata", "layout.conf"), []byte("cache-formats = md5-dict\nmasters =\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(dir, "profiles"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "profiles", "categories"), []byte("app-test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	pkgDir := filepath.Join(dir, "app-test", "test")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	ebuildContent := `EAPI=8
DESCRIPTION="Integration test"
DEPEND="
    virtual/pkgconfig
    app-arch/unzip
"
BDEPEND="
    dev-build/cmake
    dev-build/ninja
"
`
	if err := os.WriteFile(filepath.Join(pkgDir, "test-1.0.ebuild"), []byte(ebuildContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Generate Cache
	cfs := g2.NewOsCacheFS(dir)
	err := g2.GenerateCacheFS(cfs, ".", nil, g2.NewCachePolicy(g2.CacheModeCI))
	if err != nil {
		t.Fatalf("Cache generation failed: %v", err)
	}

	// Ensure cache was generated
	cachePath := filepath.Join(dir, "metadata", "md5-cache", "app-test", "test-1.0")
	cacheData, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("Cache file not created: %v", err)
	}

	if strings.Contains(string(cacheData), "\n    ") {
		t.Fatalf("Cache contains raw multiline data: %q", string(cacheData))
	}

	if !strings.Contains(string(cacheData), "DEPEND=virtual/pkgconfig app-arch/unzip") {
		t.Fatalf("Cache does not contain properly flattened DEPEND string: %q", string(cacheData))
	}
	if !strings.Contains(string(cacheData), "BDEPEND=dev-build/cmake dev-build/ninja") {
		t.Fatalf("Cache does not contain properly flattened BDEPEND string: %q", string(cacheData))
	}

	// 2. Direct Lint Integration (verifying specifically MD5CacheInvalidLintRule)
	rule := &md5cache.MD5CacheInvalidLintRule{}
	pkg := &g2.PackageData{
		Category: "app-test",
		Name:     "test",
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				PVR:     "1.0",
				Ebuild:  &g2.Ebuild{Path: filepath.Join(pkgDir, "test-1.0.ebuild")},
			},
		},
	}

	results := rule.Lint(dir, pkg)
	if len(results) > 0 {
		for _, r := range results {
			t.Logf("Unexpected Lint Result: %s", r.Message)
		}
		t.Fatalf("Expected md5cache rule to report 0 errors for properly formatted cache, got %d", len(results))
	}

	// 3. Verify Cache
	policy := g2.NewCachePolicy(g2.CacheModeCI)
	// Run user-facing verification workflow
	err = doCacheVerify(cfs, ".", policy)
	if err != nil {
		t.Fatalf("Cache verification via doCacheVerify failed: %v", err)
	}

	ebuild, err := g2.ParseEbuild(cfs, filepath.Join("app-test", "test", "test-1.0.ebuild"), g2.ParseFull)
	if err != nil {
		t.Fatalf("Failed to parse ebuild for verification: %v", err)
	}
	resolver, _ := g2.BuildEclassResolver(cfs, ".", policy)
	expected, status, err := g2.GetExpectedCacheContent(cfs, filepath.Join("app-test", "test", "test-1.0.ebuild"), ebuild, policy, resolver)
	if err != nil {
		t.Fatalf("Failed to get expected cache content: %v", err)
	}

	if status != g2.CacheDrift {
		t.Fatalf("Expected cache status drift (meaning we constructed it), got %v", status)
	}

	verifyRes := g2.CompareCacheEntry(cfs, filepath.Join("metadata", "md5-cache", "app-test", "test-1.0"), expected)
	if verifyRes.Status != g2.CacheVerified {
		t.Fatalf("Cache verification failed: %v", verifyRes.Message)
	}

	// 4. Reconcile
	baseCfs := g2.NewOsCacheFS(dir)
	spy := &SpyCacheFS{CacheFS: baseCfs}

	err = doCacheReconcile(spy, ".", policy)
	if err != nil {
		t.Fatalf("First reconcile failed: %v", err)
	}

	spy.creates = 0
	spy.removes = 0
	spy.removesAll = 0

	err = doCacheReconcile(spy, ".", policy)
	if err != nil {
		t.Fatalf("Second reconcile failed: %v", err)
	}

	if spy.creates > 0 || spy.removes > 0 || spy.removesAll > 0 {
		t.Fatalf("Cache was mutated on second run (reconciliation failed zero-mutation check): %d creates, %d removes, %d removesAll", spy.creates, spy.removes, spy.removesAll)
	}
}
