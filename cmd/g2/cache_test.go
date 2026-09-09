package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"github.com/arran4/g2"
)

func setupTestRepo(t *testing.T) (string, g2.CacheFS) {
	dir := t.TempDir()

	// Create layout.conf
	_ = os.MkdirAll(filepath.Join(dir, "metadata"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "metadata", "layout.conf"), []byte("cache-formats = md5-dict\n"), 0644)

	// Create an ebuild
	_ = os.MkdirAll(filepath.Join(dir, "sys-apps", "test"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "sys-apps", "test", "test-1.0.ebuild"), []byte("DESCRIPTION=\"A test ebuild\"\nSLOT=\"0\"\n"), 0644)

	return dir, g2.NewOsCacheFS(dir)
}

func TestDoCacheReconcile(t *testing.T) {
	dir, cfs := setupTestRepo(t)

	// 1. Create a legacy dict entry
	legacyDir := filepath.Join(dir, "metadata", "md5-dict", "sys-apps")
	_ = os.MkdirAll(legacyDir, 0755)
	_ = os.WriteFile(filepath.Join(legacyDir, "test-1.0"), []byte("DESCRIPTION=A test ebuild\n_md5_=legacy\n"), 0644)

	// 2. Create an orphan cache entry
	orphanDir := filepath.Join(dir, "metadata", "md5-cache", "sys-apps")
	_ = os.MkdirAll(orphanDir, 0755)
	_ = os.WriteFile(filepath.Join(orphanDir, "orphan-1.0"), []byte("DESCRIPTION=Orphan\n_md5_=123\n"), 0644)

	// Run reconcile
	err := doCacheReconcile(cfs, ".")
	if err != nil {
		t.Fatalf("Expected reconcile to succeed, got %v", err)
	}

	// Verify legacy is gone
	if _, err := os.Stat(filepath.Join(dir, "metadata", "md5-dict")); !os.IsNotExist(err) {
		t.Errorf("Expected legacy directory to be removed")
	}

	// Verify orphan is gone
	if _, err := os.Stat(filepath.Join(dir, "metadata", "md5-cache", "sys-apps", "orphan-1.0")); !os.IsNotExist(err) {
		t.Errorf("Expected orphan cache entry to be removed")
	}

	// Verify generated is present
	if _, err := os.Stat(filepath.Join(dir, "metadata", "md5-cache", "sys-apps", "test-1.0")); os.IsNotExist(err) {
		t.Errorf("Expected generated cache entry to be present")
	}

	// Idempotency check: run it again, it should still succeed
	err = doCacheReconcile(cfs, ".")
	if err != nil {
		t.Fatalf("Expected second reconcile to succeed idempotently, got %v", err)
	}
}

func TestDoCacheVerify(t *testing.T) {
	dir, cfs := setupTestRepo(t)

	// Verify should fail initially (missing cache)
	err := doCacheVerify(cfs, ".")
	if err == nil {
		t.Errorf("Expected verify to fail due to missing cache")
	}

	// Generate cache correctly
	_ = g2.GenerateCacheFS(cfs, ".", nil, false)

	// Verify should succeed now
	err = doCacheVerify(cfs, ".")
	if err != nil {
		t.Errorf("Expected verify to succeed, got %v", err)
	}

	// Create _md5_ mismatch
	cachePath := filepath.Join(dir, "metadata", "md5-cache", "sys-apps", "test-1.0")
	cacheBytes, _ := os.ReadFile(cachePath)
	badBytes := strings.Replace(string(cacheBytes), "_md5_=", "_md5_=bad", 1)
	_ = os.WriteFile(cachePath, []byte(badBytes), 0644)

	err = doCacheVerify(cfs, ".")
	if err == nil {
		t.Errorf("Expected verify to fail due to md5 mismatch")
	}

	// Reset cache
	_ = g2.GenerateCacheFS(cfs, ".", nil, false)

	// Create orphan
	_ = os.WriteFile(filepath.Join(dir, "metadata", "md5-cache", "sys-apps", "orphan-2.0"), []byte(""), 0644)
	err = doCacheVerify(cfs, ".")
	if err == nil {
		t.Errorf("Expected verify to fail due to orphan entry")
	}
	_ = os.Remove(filepath.Join(dir, "metadata", "md5-cache", "sys-apps", "orphan-2.0"))

	// Create legacy
	_ = os.MkdirAll(filepath.Join(dir, "metadata", "md5-dict"), 0755)
	err = doCacheVerify(cfs, ".")
	if err == nil {
		t.Errorf("Expected verify to fail due to legacy directory")
	}
}
