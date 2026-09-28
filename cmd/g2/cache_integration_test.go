package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/md5cache"
)

func TestCacheIntegration_MultilineLiteral(t *testing.T) {
	// Setup repo
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
	inputFS := fstest.MapFS{
		"metadata/layout.conf":          &fstest.MapFile{Data: []byte("cache-formats = md5-dict\nmasters =\n")},
		"profiles/categories":           &fstest.MapFile{Data: []byte("app-test\n")},
		"app-test/test/test-1.0.ebuild": &fstest.MapFile{Data: []byte(ebuildContent)},
	}

	// 1. Generate Cache
	baseFS := NewMemCacheFS(inputFS)
	cfs := &SpyCacheFS{CacheFS: baseFS}
	err := g2.GenerateCacheFS(cfs, ".", nil, g2.NewCachePolicy(g2.CacheModeCI))
	if err != nil {
		t.Fatalf("Cache generation failed: %v", err)
	}

	// Ensure cache was generated
	cachePath := filepath.ToSlash(filepath.Join("metadata", "md5-cache", "app-test", "test-1.0"))
	cacheData, err := fs.ReadFile(baseFS, cachePath)
	if err != nil {
		t.Fatalf("Cache file not created: %v", err)
	}

	if strings.Contains(string(cacheData), "\n    ") {
		t.Fatalf("Cache contains raw multiline data: %q", string(cacheData))
	}

	cacheStr := string(cacheData)
	lines := strings.Split(cacheStr, "\n")
	foundDepend := false
	foundBdepend := false
	for _, line := range lines {
		if line == "DEPEND=virtual/pkgconfig app-arch/unzip" {
			foundDepend = true
		}
		if line == "BDEPEND=dev-build/cmake dev-build/ninja" {
			foundBdepend = true
		}
	}

	if !foundDepend {
		t.Fatalf("Cache does not contain exactly flattened DEPEND line: %q", cacheStr)
	}
	if !foundBdepend {
		t.Fatalf("Cache does not contain exactly flattened BDEPEND line: %q", cacheStr)
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
				Ebuild:  &g2.Ebuild{Path: filepath.ToSlash(filepath.Join("app-test", "test", "test-1.0.ebuild"))},
			},
		},
	}

	results := rule.LintFS(baseFS, ".", pkg, nil, nil)
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

	vars := g2.ParseEbuildVariables("test-1.0.ebuild")
	ident := g2.GetCacheIdentity(".", "md5-dict", "app-test", "test", g2.VersionData{Version: vars["PV"], PVR: vars["PVR"], Ebuild: ebuild})

	expected, status, err := g2.GetExpectedCacheContent(cfs, ident, ebuild, policy, resolver)
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
	spy := &SpyCacheFS{CacheFS: cfs}

	err = doCacheReconcile(spy, ".", policy)
	if err != nil {
		t.Fatalf("First reconcile failed: %v", err)
	}

	spy.creates = 0
	spy.removes = 0
	spy.removesAll = 0
	spy.MkdirAlls = 0

	err = doCacheReconcile(spy, ".", policy)
	if err != nil {
		t.Fatalf("Second reconcile failed: %v", err)
	}

	if spy.creates > 0 || spy.removes > 0 || spy.removesAll > 0 || spy.MkdirAlls > 0 {
		t.Fatalf("Cache was mutated on second run (reconciliation failed zero-mutation check): %d creates, %d removes, %d removesAll, %d mkdirAlls", spy.creates, spy.removes, spy.removesAll, spy.MkdirAlls)
	}
}

// MemCacheFS implements CacheFS for testing
type MemCacheFS struct {
	fs.FS
	Map        fstest.MapFS
	Creates    []string
	Removes    []string
	RemoveAlls []string
	MkdirAlls  []string
	Writes     []string
}

func NewMemCacheFS(m fstest.MapFS) *MemCacheFS {
	return &MemCacheFS{
		FS:  m,
		Map: m,
	}
}

func (m *MemCacheFS) MkdirAll(path string, perm os.FileMode) error {
	m.MkdirAlls = append(m.MkdirAlls, path)
	parts := strings.Split(filepath.ToSlash(path), "/")
	var current string
	for _, p := range parts {
		if p == "" {
			continue
		}
		if current == "" {
			current = p
		} else {
			current = current + "/" + p
		}
		// If it does not exist, create it as a directory. Ignore if exists.
		if _, ok := m.Map[current]; !ok {
			m.Map[current] = &fstest.MapFile{Mode: perm | fs.ModeDir}
		}
	}
	return nil
}

type memFile struct {
	name string
	buf  *bytes.Buffer
	m    *MemCacheFS
}

func (f *memFile) Write(p []byte) (n int, err error) {
	f.m.Writes = append(f.m.Writes, f.name)
	return f.buf.Write(p)
}

func (f *memFile) Close() error {
	f.m.Map[f.name] = &fstest.MapFile{Data: f.buf.Bytes()}
	return nil
}

func (m *MemCacheFS) Create(name string) (io.WriteCloser, error) {
	m.Creates = append(m.Creates, name)
	return &memFile{
		name: name,
		buf:  new(bytes.Buffer),
		m:    m,
	}, nil
}

func (m *MemCacheFS) RemoveAll(name string) error {
	m.RemoveAlls = append(m.RemoveAlls, name)
	for k := range m.Map {
		if k == name || (len(k) > len(name) && k[:len(name)+1] == name+"/") {
			delete(m.Map, k)
		}
	}
	return nil
}

func (m *MemCacheFS) Remove(name string) error {
	m.Removes = append(m.Removes, name)
	if _, ok := m.Map[name]; !ok {
		return os.ErrNotExist
	}
	delete(m.Map, name)
	return nil
}

func (m *MemCacheFS) Walk(root string, fn fs.WalkDirFunc) error {
	return fs.WalkDir(m.FS, root, fn)
}

func (m *MemCacheFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(m.Map, name)
}

type errorReadFS struct {
	g2.CacheFS
	failPath string
}

func (e errorReadFS) Open(name string) (fs.File, error) {
	if name == e.failPath {
		return nil, fs.ErrPermission // Simulated unexpected read error
	}
	return e.CacheFS.(fs.FS).Open(name)
}

func TestCacheIntegration_ExistingValidCache_ReadFailure(t *testing.T) {
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
	validCacheContent := "DEPEND=virtual/pkgconfig app-arch/unzip\n_md5_=dummy"
	inputFS := fstest.MapFS{
		"metadata/layout.conf":                 &fstest.MapFile{Data: []byte("cache-formats = md5-dict\nmasters =\n")},
		"profiles/categories":                  &fstest.MapFile{Data: []byte("app-test\n")},
		"app-test/test/test-1.0.ebuild":        &fstest.MapFile{Data: []byte(ebuildContent)},
		"metadata/md5-cache/app-test/test-1.0": &fstest.MapFile{Data: []byte(validCacheContent)},
	}

	baseFS := NewMemCacheFS(inputFS)
	spyFS := &SpyCacheFS{CacheFS: baseFS}
	errorFS := &errorReadFS{CacheFS: spyFS, failPath: "metadata/md5-cache/app-test/test-1.0"}

	snapshotBefore := snapshotMapFS(baseFS.Map)
	err := g2.GenerateCacheFS(errorFS, ".", nil, g2.NewCachePolicy(g2.CacheModeCI))
	if err == nil {
		t.Fatalf("Expected GenerateCacheFS to fail due to unexpected read error")
	}

	if !strings.Contains(err.Error(), "reading existing cache file") {
		t.Fatalf("Expected error propagating read failure, got: %v", err)
	}

	// Verify the cache content was NOT changed and no writes were attempted
	cachePath := filepath.ToSlash(filepath.Join("metadata", "md5-cache", "app-test", "test-1.0"))
	cacheData, _ := fs.ReadFile(baseFS, cachePath)
	if string(cacheData) != validCacheContent {
		t.Fatalf("Existing valid cache was modified!")
	}
	if spyFS.creates > 0 || spyFS.removes > 0 || spyFS.removesAll > 0 || spyFS.MkdirAlls > 0 {
		t.Fatalf("Virtual filesystem mutated! creates: %d, removes: %d, removesAll: %d, mkdirAlls: %d", spyFS.creates, spyFS.removes, spyFS.removesAll, spyFS.MkdirAlls)
	}

	assertMapFSSnapshotEqual(t, snapshotBefore, snapshotMapFS(baseFS.Map))

}

func TestCacheIntegration_AbsentCacheDirectory_UnsupportedMetadata(t *testing.T) {
	ebuildContent := `EAPI=8
DESCRIPTION="Integration test"
DEPEND="
    virtual/pkgconfig
    app-arch/unzip
"
BDEPEND="
    dev-build/cmake
    $(some_dynamic_function)
"
`
	inputFS := fstest.MapFS{
		"metadata/layout.conf":          &fstest.MapFile{Data: []byte("cache-formats = md5-dict\nmasters =\n")},
		"profiles/categories":           &fstest.MapFile{Data: []byte("app-test\n")},
		"app-test/test/test-1.0.ebuild": &fstest.MapFile{Data: []byte(ebuildContent)},
	}

	baseFS := NewMemCacheFS(inputFS)
	cfs := &SpyCacheFS{CacheFS: baseFS}
	snapshotBefore := snapshotMapFS(baseFS.Map)
	err := g2.GenerateCacheFS(cfs, ".", nil, g2.NewCachePolicy(g2.CacheModeCI))
	if err == nil {
		t.Fatalf("Expected GenerateCacheFS to fail due to unsupported metadata")
	}

	// Verify the cache directory was NOT created
	cacheDir := filepath.ToSlash(filepath.Join("metadata", "md5-cache", "app-test"))
	if _, ok := baseFS.Map[cacheDir]; ok {
		t.Fatalf("Cache directory %s was created prematurely!", cacheDir)
	}
	if cfs.creates > 0 || cfs.removes > 0 || cfs.removesAll > 0 || cfs.MkdirAlls > 0 {
		t.Fatalf("Virtual filesystem mutated! creates: %d, removes: %d, removesAll: %d, mkdirAlls: %d", cfs.creates, cfs.removes, cfs.removesAll, cfs.MkdirAlls)
	}
	assertMapFSSnapshotEqual(t, snapshotBefore, snapshotMapFS(baseFS.Map))
}

func TestCacheIntegration_ExistingValidCache_UnsupportedMetadata(t *testing.T) {
	ebuildContent := `EAPI=8
DESCRIPTION="Integration test"
DEPEND="
    virtual/pkgconfig
    app-arch/unzip
"
BDEPEND="
    dev-build/cmake
    $(some_dynamic_function)
"
`
	validCacheContent := "DEPEND=virtual/pkgconfig app-arch/unzip\n_md5_=dummy"
	inputFS := fstest.MapFS{
		"metadata/layout.conf":                 &fstest.MapFile{Data: []byte("cache-formats = md5-dict\nmasters =\n")},
		"profiles/categories":                  &fstest.MapFile{Data: []byte("app-test\n")},
		"app-test/test/test-1.0.ebuild":        &fstest.MapFile{Data: []byte(ebuildContent)},
		"metadata/md5-cache/app-test/test-1.0": &fstest.MapFile{Data: []byte(validCacheContent)},
	}

	baseFS := NewMemCacheFS(inputFS)
	cfs := &SpyCacheFS{CacheFS: baseFS}
	snapshotBefore := snapshotMapFS(baseFS.Map)
	err := g2.GenerateCacheFS(cfs, ".", nil, g2.NewCachePolicy(g2.CacheModeCI))
	if err == nil {
		t.Fatalf("Expected GenerateCacheFS to fail due to unsupported metadata")
	}

	// Verify the cache content was NOT changed
	cachePath := filepath.ToSlash(filepath.Join("metadata", "md5-cache", "app-test", "test-1.0"))
	cacheData, _ := fs.ReadFile(baseFS, cachePath)
	if string(cacheData) != validCacheContent {
		t.Fatalf("Existing valid cache was modified!")
	}
	if cfs.creates > 0 || cfs.removes > 0 || cfs.removesAll > 0 || cfs.MkdirAlls > 0 {
		t.Fatalf("Virtual filesystem mutated! creates: %d, removes: %d, removesAll: %d, mkdirAlls: %d", cfs.creates, cfs.removes, cfs.removesAll, cfs.MkdirAlls)
	}
	assertMapFSSnapshotEqual(t, snapshotBefore, snapshotMapFS(baseFS.Map))
}

func TestCacheIntegration_UnchangedValidEntry_RepeatedReconciliation(t *testing.T) {
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
	inputFS := fstest.MapFS{
		"metadata/layout.conf":          &fstest.MapFile{Data: []byte("cache-formats = md5-dict\nmasters =\n")},
		"profiles/categories":           &fstest.MapFile{Data: []byte("app-test\n")},
		"app-test/test/test-1.0.ebuild": &fstest.MapFile{Data: []byte(ebuildContent)},
	}

	baseFS := NewMemCacheFS(inputFS)
	cfs := &SpyCacheFS{CacheFS: baseFS}
	// Initial Generation
	err := g2.GenerateCacheFS(cfs, ".", nil, g2.NewCachePolicy(g2.CacheModeCI))
	if err != nil {
		t.Fatalf("Cache generation failed: %v", err)
	}

	policy := g2.NewCachePolicy(g2.CacheModeCI)

	// First Reconcile
	spy := &SpyCacheFS{CacheFS: cfs}
	err = doCacheReconcile(spy, ".", policy)
	if err != nil {
		t.Fatalf("First reconcile failed: %v", err)
	}

	// Second Reconcile
	spy.creates = 0
	spy.removes = 0
	spy.removesAll = 0
	spy.MkdirAlls = 0
	snapshotBefore := snapshotMapFS(baseFS.Map)
	err = doCacheReconcile(spy, ".", policy)
	if err != nil {
		t.Fatalf("Second reconcile failed: %v", err)
	}

	if spy.creates > 0 || spy.removes > 0 || spy.removesAll > 0 || spy.MkdirAlls > 0 {
		t.Fatalf("Repeated reconciliation mutated cache: %d creates, %d removes, %d removesAll, %d mkdirAlls", spy.creates, spy.removes, spy.removesAll, spy.MkdirAlls)
	}
	assertMapFSSnapshotEqual(t, snapshotBefore, snapshotMapFS(baseFS.Map))
}

func snapshotMapFS(m fstest.MapFS) map[string]string {
	snap := make(map[string]string)
	for k, v := range m {
		snap[k] = string(v.Data) + "|" + string(rune(v.Mode))
	}
	return snap
}

func assertMapFSSnapshotEqual(t *testing.T, before, after map[string]string) {
	for k, vBefore := range before {
		if vAfter, ok := after[k]; !ok {
			t.Errorf("File %s was deleted from snapshot", k)
		} else if vBefore != vAfter {
			t.Errorf("File %s content/mode changed: %q -> %q", k, vBefore, vAfter)
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			t.Errorf("File %s was newly added to snapshot", k)
		}
	}
}

func TestCacheIntegration_EclassHashFallback(t *testing.T) {
	ebuildContent := `EAPI=8
DESCRIPTION="Integration test eclass"
DEPEND="virtual/pkgconfig"
INHERITED="test-eclass"
`
	eclassContent := `# testing eclass`
	validCacheContent := "DEPEND=virtual/pkgconfig\nINHERITED=test-eclass\n_eclasses_=test-eclass\t" + fmt.Sprintf("%x", md5.Sum([]byte(eclassContent))) + "\n_md5_=" + fmt.Sprintf("%x", md5.Sum([]byte(ebuildContent)))

	inputFS := fstest.MapFS{
		"metadata/layout.conf":                 &fstest.MapFile{Data: []byte("cache-formats = md5-dict\nmasters =\n")},
		"profiles/categories":                  &fstest.MapFile{Data: []byte("app-test\n")},
		"eclass/test-eclass.eclass":            &fstest.MapFile{Data: []byte(eclassContent)},
		"app-test/test/test-1.0.ebuild":        &fstest.MapFile{Data: []byte(ebuildContent)},
		"metadata/md5-cache/app-test/test-1.0": &fstest.MapFile{Data: []byte(validCacheContent)},
	}

	baseFS := NewMemCacheFS(inputFS)

	// We pass nil for hashEclass to test the native fallback implementation
	rule := &md5cache.MD5CacheInvalidLintRule{}
	pkg := &g2.PackageData{
		Category: "app-test",
		Name:     "test",
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				PVR:     "1.0",
				Ebuild:  &g2.Ebuild{Path: filepath.ToSlash(filepath.Join("app-test", "test", "test-1.0.ebuild"))},
			},
		},
	}

	results := rule.LintFS(baseFS, ".", pkg, nil, nil)
	if len(results) > 0 {
		for _, r := range results {
			t.Errorf("Unexpected Lint Result: %s", r.Message)
		}
		t.Fatalf("Expected 0 errors for eclass lint fallback, got %d", len(results))

	}
}
