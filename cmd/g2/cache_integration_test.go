package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arran4/g2"
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

	// 2. Run Lint (simulating g2 lint)
	cfg := &MainArgConfig{}

	// We will capture stdout
	out, errLint := captureStdout(t, func() error {
		return cfg.cmdLint([]string{"--fail-severity=warning", dir})
	})

	if errLint != nil {
		t.Logf("cmdLint returned error (expected since mock repo fails validations): %v", errLint)
	}

	if strings.Contains(out, "[Warning] Invalid format in md5-cache") {
		t.Fatalf("g2 lint reported invalid format: %s", out)
	}

	// 3. Verify Cache
	policy := g2.NewCachePolicy(g2.CacheModeCI)
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
	// To test zero-mutation, check modtime before and after GenerateCacheFS (which handles reconciliation)
	statBefore, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}

	err = g2.GenerateCacheFS(cfs, ".", nil, policy)
	if err != nil {
		t.Fatalf("Second cache generation failed: %v", err)
	}

	statAfter, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}

	if !os.SameFile(statBefore, statAfter) {
		t.Fatalf("Cache was mutated on second run (reconciliation failed zero-mutation check)")
	}
	if statBefore.ModTime() != statAfter.ModTime() {
		t.Fatalf("Cache modification time changed on second run")
	}
}
