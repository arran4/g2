package main

import (
	"github.com/arran4/g2"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func snapshotDir(t *testing.T, dir string) map[string]string {
	snapshot := make(map[string]string)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snapshot[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot error: %v", err)
	}
	return snapshot
}

func assertSnapshotEqual(t *testing.T, snap1, snap2 map[string]string) {
	for k, v1 := range snap1 {
		if v2, ok := snap2[k]; !ok {
			t.Errorf("file %s is missing in second snapshot", k)
		} else if v1 != v2 {
			t.Errorf("file %s content differs", k)
		}
	}
	for k := range snap2 {
		if _, ok := snap1[k]; !ok {
			t.Errorf("file %s is unexpectedly present in second snapshot", k)
		}
	}
}

type SpyCacheFS struct {
	g2.CacheFS
	creates    int
	removes    int
	removesAll int
}

func (s *SpyCacheFS) Create(name string) (io.WriteCloser, error) {
	s.creates++
	return s.CacheFS.Create(name)
}

func (s *SpyCacheFS) Remove(name string) error {
	s.removes++
	return s.CacheFS.Remove(name)
}

func (s *SpyCacheFS) RemoveAll(name string) error {
	s.removesAll++
	return s.CacheFS.RemoveAll(name)
}

func setupTestRepo(t *testing.T) (string, g2.CacheFS) {
	dir := t.TempDir()

	// Create layout.conf
	if err := os.MkdirAll(filepath.Join(dir, "metadata"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata", "layout.conf"), []byte("cache-formats = md5-dict\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Create an ebuild
	if err := os.MkdirAll(filepath.Join(dir, "sys-apps", "test"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sys-apps", "test", "test-1.0.ebuild"), []byte("DESCRIPTION=\"A test ebuild\"\nSLOT=\"0\"\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	return dir, g2.NewOsCacheFS(dir)
}

func TestDoCacheReconcile(t *testing.T) {
	dir, baseCfs := setupTestRepo(t)
	cfs := &SpyCacheFS{CacheFS: baseCfs}

	// 1. Create a legacy dict entry
	legacyDir := filepath.Join(dir, "metadata", "md5-dict", "sys-apps")
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "test-1.0"), []byte("DESCRIPTION=A test ebuild\n_md5_=legacy\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// 2. Create an orphan cache entry
	orphanDir := filepath.Join(dir, "metadata", "md5-cache", "sys-apps")
	if err := os.MkdirAll(orphanDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orphanDir, "orphan-1.0"), []byte("DESCRIPTION=Orphan\n_md5_=123\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

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
	snap1 := snapshotDir(t, dir)

	// Reset counters
	cfs.creates = 0
	cfs.removes = 0
	cfs.removesAll = 0

	err = doCacheReconcile(cfs, ".")
	if err != nil {
		t.Fatalf("Expected second reconcile to succeed idempotently, got %v", err)
	}
	snap2 := snapshotDir(t, dir)
	assertSnapshotEqual(t, snap1, snap2)

	// Assert ZERO mutations
	if cfs.creates > 0 || cfs.removes > 0 || cfs.removesAll > 0 {
		t.Fatalf("Expected 0 mutations on idempotent run, got %d creates, %d removes, %d removesAll", cfs.creates, cfs.removes, cfs.removesAll)
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
	if err := g2.GenerateCacheFS(cfs, ".", nil, false); err != nil {
		t.Fatalf("GenerateCacheFS failed: %v", err)
	}

	// Verify should succeed now
	err = doCacheVerify(cfs, ".")
	if err != nil {
		t.Errorf("Expected verify to succeed, got %v", err)
	}

	// Create _md5_ mismatch
	cachePath := filepath.Join(dir, "metadata", "md5-cache", "sys-apps", "test-1.0")
	cacheBytes, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	badBytes := strings.Replace(string(cacheBytes), "_md5_=", "_md5_=bad", 1)
	if err := os.WriteFile(cachePath, []byte(badBytes), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	err = doCacheVerify(cfs, ".")
	if err == nil {
		t.Errorf("Expected verify to fail due to md5 mismatch")
	}

	// Reset cache
	if err := g2.GenerateCacheFS(cfs, ".", nil, false); err != nil {
		t.Fatalf("GenerateCacheFS failed: %v", err)
	}

	// Create orphan
	if err := os.WriteFile(filepath.Join(dir, "metadata", "md5-cache", "sys-apps", "orphan-2.0"), []byte(""), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	snapBefore := snapshotDir(t, dir)
	err = doCacheVerify(cfs, ".")
	if err == nil {
		t.Errorf("Expected verify to fail due to orphan entry")
	}
	snapAfter := snapshotDir(t, dir)
	assertSnapshotEqual(t, snapBefore, snapAfter)

	if err := os.Remove(filepath.Join(dir, "metadata", "md5-cache", "sys-apps", "orphan-2.0")); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Create legacy
	if err := os.MkdirAll(filepath.Join(dir, "metadata", "md5-dict"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	err = doCacheVerify(cfs, ".")
	if err == nil {
		t.Errorf("Expected verify to fail due to legacy directory")
	}
}

func TestCacheUnknownFormat(t *testing.T) {
	dir, cfs := setupTestRepo(t)
	// Set layout to an unknown format
	if err := os.WriteFile(filepath.Join(dir, "metadata", "layout.conf"), []byte("cache-formats = invalid-format\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Clean should skip invalid-format and not create metadata/invalid-format
	if err := doCacheClean(cfs, "."); err != nil {
		t.Fatalf("clean failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "metadata", "invalid-format")); !os.IsNotExist(err) {
		t.Fatalf("clean incorrectly created or operated on metadata/invalid-format")
	}

	// Generate should skip invalid-format
	if err := g2.GenerateCacheFS(cfs, ".", nil, false); err != nil {
		t.Fatalf("GenerateCacheFS failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "metadata", "invalid-format")); !os.IsNotExist(err) {
		t.Fatalf("generate incorrectly created metadata/invalid-format")
	}
}

func TestCacheVerifyLayoutError(t *testing.T) {
	dir, cfs := setupTestRepo(t)

	if err := os.WriteFile(filepath.Join(dir, "metadata", "layout.conf"), []byte("invalid syntax that cannot parse\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	err := doCacheVerify(cfs, ".")
	if err == nil {
		t.Fatalf("Expected verify to fail when layout.conf is unparseable")
	}
	if !strings.Contains(err.Error(), "failed to parse layout.conf") {
		t.Fatalf("Expected layout.conf parse error, got: %v", err)
	}
}

type failingOpenFS struct {
	g2.CacheFS
}

func (f *failingOpenFS) Open(name string) (fs.File, error) {
	if name == filepath.ToSlash(filepath.Join(".", "metadata", "layout.conf")) || name == "metadata/layout.conf" {
		return nil, os.ErrPermission
	}
	return f.CacheFS.Open(name)
}

func TestCacheVerifyLayoutOpenError(t *testing.T) {
	_, cfs := setupTestRepo(t)

	errFS := &failingOpenFS{CacheFS: cfs}
	err := doCacheVerify(errFS, ".")
	if err == nil {
		t.Fatalf("Expected verify to fail when layout.conf cannot be opened")
	}
	if !strings.Contains(err.Error(), "failed to open layout.conf") {
		t.Fatalf("Expected layout.conf open error, got: %v", err)
	}
}
