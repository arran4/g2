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

func TestCacheRevisionedEbuilds(t *testing.T) {
	dir := t.TempDir()

	// Create layout.conf
	if err := os.MkdirAll(filepath.Join(dir, "metadata"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata", "layout.conf"), []byte("cache-formats = md5-dict\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// 1. Revisioned ebuild with non-zero PV: sys-apps/test/test-1.2.3-r1.ebuild
	if err := os.MkdirAll(filepath.Join(dir, "sys-apps", "test"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	testEbuildContent := "DESCRIPTION=\"Revisioned test package\"\nSLOT=\"0\"\n"
	if err := os.WriteFile(filepath.Join(dir, "sys-apps", "test", "test-1.2.3-r1.ebuild"), []byte(testEbuildContent), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// 2. Revisioned ebuild with zero PV: acct-group/ollama/ollama-0-r1.ebuild
	if err := os.MkdirAll(filepath.Join(dir, "acct-group", "ollama"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	ollamaEbuildContent := "DESCRIPTION=\"Revisioned zero PV package\"\nSLOT=\"0\"\n"
	if err := os.WriteFile(filepath.Join(dir, "acct-group", "ollama", "ollama-0-r1.ebuild"), []byte(ollamaEbuildContent), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// 3. Unrevised ebuild: dev-libs/unrevised/unrevised-2.0.ebuild
	if err := os.MkdirAll(filepath.Join(dir, "dev-libs", "unrevised"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	unrevisedContent := "DESCRIPTION=\"Unrevised package\"\nSLOT=\"0\"\n"
	if err := os.WriteFile(filepath.Join(dir, "dev-libs", "unrevised", "unrevised-2.0.ebuild"), []byte(unrevisedContent), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfs := g2.NewOsCacheFS(dir)

	// Verify should fail before generation
	if err := doCacheVerify(cfs, "."); err == nil {
		t.Fatalf("Expected verify to fail prior to cache generation")
	}

	// Generate cache
	if err := g2.GenerateCacheFS(cfs, ".", nil, false); err != nil {
		t.Fatalf("GenerateCacheFS failed: %v", err)
	}

	// Verify expected files exist
	expectedFiles := []string{
		filepath.Join(dir, "metadata", "md5-cache", "sys-apps", "test-1.2.3-r1"),
		filepath.Join(dir, "metadata", "md5-cache", "acct-group", "ollama-0-r1"),
		filepath.Join(dir, "metadata", "md5-cache", "dev-libs", "unrevised-2.0"),
	}
	for _, f := range expectedFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("Expected cache file %s does not exist", f)
		}
	}

	// Verify unwanted files DO NOT exist
	unwantedFiles := []string{
		filepath.Join(dir, "metadata", "md5-cache", "sys-apps", "test-1.2.3"),
		filepath.Join(dir, "metadata", "md5-cache", "acct-group", "ollama-0"),
		filepath.Join(dir, "metadata", "md5-cache", "dev-libs", "unrevised-2.0-r0"),
	}
	for _, f := range unwantedFiles {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("Unwanted cache file %s unexpectedly exists", f)
		}
	}

	// Verification should succeed cleanly
	if err := doCacheVerify(cfs, "."); err != nil {
		t.Fatalf("Expected doCacheVerify to succeed cleanly, got: %v", err)
	}

	// Verify that MD5 mismatch on revisioned ebuild is detected
	if err := os.WriteFile(filepath.Join(dir, "sys-apps", "test", "test-1.2.3-r1.ebuild"), []byte("DESCRIPTION=\"Mutated content\"\nSLOT=\"0\"\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := doCacheVerify(cfs, "."); err == nil {
		t.Fatalf("Expected doCacheVerify to fail after mutating revisioned ebuild")
	}
}

func TestCacheCleanRevisioned(t *testing.T) {
	dir := t.TempDir()

	// Create layout.conf
	if err := os.MkdirAll(filepath.Join(dir, "metadata"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata", "layout.conf"), []byte("cache-formats = md5-dict\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Only pkg-1.0-r2.ebuild exists in the tree
	if err := os.MkdirAll(filepath.Join(dir, "sys-apps", "pkg"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sys-apps", "pkg", "pkg-1.0-r2.ebuild"), []byte("DESCRIPTION=\"Pkg\"\nSLOT=\"0\"\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// In metadata/md5-cache/sys-apps, populate current revision and stale sibling revisions
	cacheDir := filepath.Join(dir, "metadata", "md5-cache", "sys-apps")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	_ = os.WriteFile(filepath.Join(cacheDir, "pkg-1.0-r2"), []byte("DESCRIPTION=Pkg\n_md5_=valid\n"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "pkg-1.0-r1"), []byte("DESCRIPTION=Pkg\n_md5_=stale-r1\n"), 0644)
	_ = os.WriteFile(filepath.Join(cacheDir, "pkg-1.0"), []byte("DESCRIPTION=Pkg\n_md5_=stale-r0\n"), 0644)

	cfs := g2.NewOsCacheFS(dir)
	if err := doCacheClean(cfs, "."); err != nil {
		t.Fatalf("doCacheClean failed: %v", err)
	}

	// Current revision pkg-1.0-r2 must be preserved
	if _, err := os.Stat(filepath.Join(cacheDir, "pkg-1.0-r2")); os.IsNotExist(err) {
		t.Errorf("Expected current revision pkg-1.0-r2 to be preserved")
	}

	// Stale revisions pkg-1.0-r1 and pkg-1.0 must be removed
	if _, err := os.Stat(filepath.Join(cacheDir, "pkg-1.0-r1")); !os.IsNotExist(err) {
		t.Errorf("Expected stale revision pkg-1.0-r1 to be removed")
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "pkg-1.0")); !os.IsNotExist(err) {
		t.Errorf("Expected stale unrevisioned pkg-1.0 to be removed")
	}
}

func TestCacheReconcileRevisionedIdempotency(t *testing.T) {
	dir := t.TempDir()

	// Create layout.conf
	if err := os.MkdirAll(filepath.Join(dir, "metadata"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata", "layout.conf"), []byte("cache-formats = md5-dict\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Add revisioned ebuilds
	if err := os.MkdirAll(filepath.Join(dir, "acct-group", "ollama"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "acct-group", "ollama", "ollama-0-r1.ebuild"), []byte("DESCRIPTION=\"Ollama\"\nSLOT=\"0\"\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(dir, "sys-apps", "test"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sys-apps", "test", "test-1.2.3-r1.ebuild"), []byte("DESCRIPTION=\"Test\"\nSLOT=\"0\"\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Add stale orphan and legacy entries
	if err := os.MkdirAll(filepath.Join(dir, "metadata", "md5-cache", "acct-group"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	_ = os.WriteFile(filepath.Join(dir, "metadata", "md5-cache", "acct-group", "ollama-0"), []byte("STALE\n"), 0644)

	if err := os.MkdirAll(filepath.Join(dir, "metadata", "md5-dict", "acct-group"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	_ = os.WriteFile(filepath.Join(dir, "metadata", "md5-dict", "acct-group", "ollama-0-r1"), []byte("LEGACY\n"), 0644)

	baseCfs := g2.NewOsCacheFS(dir)
	spy := &SpyCacheFS{CacheFS: baseCfs}

	// First reconcile
	if err := doCacheReconcile(spy, "."); err != nil {
		t.Fatalf("First reconcile failed: %v", err)
	}

	// Assert correct state
	if _, err := os.Stat(filepath.Join(dir, "metadata", "md5-cache", "acct-group", "ollama-0-r1")); os.IsNotExist(err) {
		t.Errorf("Expected acct-group/ollama-0-r1 to exist after reconcile")
	}
	if _, err := os.Stat(filepath.Join(dir, "metadata", "md5-cache", "sys-apps", "test-1.2.3-r1")); os.IsNotExist(err) {
		t.Errorf("Expected sys-apps/test-1.2.3-r1 to exist after reconcile")
	}
	if _, err := os.Stat(filepath.Join(dir, "metadata", "md5-cache", "acct-group", "ollama-0")); !os.IsNotExist(err) {
		t.Errorf("Expected stale acct-group/ollama-0 to be cleaned")
	}
	if _, err := os.Stat(filepath.Join(dir, "metadata", "md5-dict")); !os.IsNotExist(err) {
		t.Errorf("Expected legacy metadata/md5-dict to be cleaned")
	}

	// Idempotency: snapshot and run reconcile second time
	snap1 := snapshotDir(t, dir)
	spy.creates = 0
	spy.removes = 0
	spy.removesAll = 0

	if err := doCacheReconcile(spy, "."); err != nil {
		t.Fatalf("Second reconcile failed: %v", err)
	}

	snap2 := snapshotDir(t, dir)
	assertSnapshotEqual(t, snap1, snap2)

	if spy.creates > 0 || spy.removes > 0 || spy.removesAll > 0 {
		t.Fatalf("Expected 0 mutations on second reconcile run, got %d creates, %d removes, %d removesAll", spy.creates, spy.removes, spy.removesAll)
	}
}
