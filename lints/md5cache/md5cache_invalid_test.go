package md5cache

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

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

	results := rule.Lint(tempDir, pkg)
	if len(results) != 0 {
		t.Errorf("Expected 0 results for valid cache, got %d: %v", len(results), results)
	}

	// 2. Invalid ebuild md5
	_ = os.WriteFile(cacheFile, []byte(fmt.Sprintf("_md5_=%s\n_eclasses_=test\t%s\n", "invalidmd5", eclassMd5)), 0644)
	results = rule.Lint(tempDir, pkg)
	if len(results) != 1 {
		t.Errorf("Expected 1 result for invalid ebuild md5, got %d", len(results))
	} else if results[0].Message != fmt.Sprintf("[Warning] Incorrect _md5_ in md5-cache for foo-1.0. Expected %s, got invalidmd5", md5sum) {
		t.Errorf("Unexpected message: %s", results[0].Message)
	}

	// 3. Invalid eclass md5
	_ = os.WriteFile(cacheFile, []byte(fmt.Sprintf("_md5_=%s\n_eclasses_=test\t%s\n", md5sum, "invalidmd5")), 0644)
	results = rule.Lint(tempDir, pkg)
	if len(results) != 1 {
		t.Errorf("Expected 1 result for invalid eclass md5, got %d", len(results))
	} else if results[0].Message != fmt.Sprintf("[Warning] Incorrect eclass md5 for test in md5-cache of foo-1.0. Expected %s, got invalidmd5", eclassMd5) {
		t.Errorf("Unexpected message: %s", results[0].Message)
	}

	// 4. Missing _md5_
	_ = os.WriteFile(cacheFile, []byte(fmt.Sprintf("_eclasses_=test\t%s\n", eclassMd5)), 0644)
	results = rule.Lint(tempDir, pkg)
	if len(results) != 1 {
		t.Errorf("Expected 1 result for missing _md5_, got %d", len(results))
	} else if results[0].Message != "[Warning] Missing _md5_ in md5-cache for foo-1.0" {
		t.Errorf("Unexpected message: %s", results[0].Message)
	}

	// 5. Invalid format
	_ = os.WriteFile(cacheFile, []byte(fmt.Sprintf("_md5_=%s\ninvalidline\n", md5sum)), 0644)
	results = rule.Lint(tempDir, pkg)
	if len(results) != 1 {
		t.Errorf("Expected 1 result for invalid format, got %d", len(results))
	} else if results[0].Message != "[Warning] Invalid format in md5-cache for foo-1.0: invalidline" {
		t.Errorf("Unexpected message: %s", results[0].Message)
	}
}

func TestMD5CacheInvalidLintRule_Invalidation(t *testing.T) {
	rule := &MD5CacheInvalidLintRule{}
	tempDir := t.TempDir()

	eclassDir := filepath.Join(tempDir, "eclass")
	_ = os.MkdirAll(eclassDir, 0755)
	eclassFile := filepath.Join(eclassDir, "test.eclass")

	baseTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	// 1. Initial state
	content1 := []byte("v1")
	_ = os.WriteFile(eclassFile, content1, 0644)
	_ = os.Chtimes(eclassFile, baseTime, baseTime)

	hash1, err := rule.getEclassMD5(eclassFile)
	if err != nil || hash1 != fmt.Sprintf("%x", md5.Sum(content1)) {
		t.Fatalf("Failed initial hash")
	}

	// 2. Different size (content changes)
	content2 := []byte("v2_longer")
	_ = os.WriteFile(eclassFile, content2, 0644)
	_ = os.Chtimes(eclassFile, baseTime, baseTime)

	hash2, err := rule.getEclassMD5(eclassFile)
	if err != nil || hash2 != fmt.Sprintf("%x", md5.Sum(content2)) {
		t.Fatalf("Failed size invalidation")
	}

	// 3. Same size, different mtime
	content3 := []byte("v3_longer") // same len as v2_longer
	_ = os.WriteFile(eclassFile, content3, 0644)
	newTime := baseTime.Add(time.Hour)
	_ = os.Chtimes(eclassFile, newTime, newTime)

	hash3, err := rule.getEclassMD5(eclassFile)
	if err != nil || hash3 != fmt.Sprintf("%x", md5.Sum(content3)) {
		t.Fatalf("Failed mtime invalidation")
	}

	// 4. Missing file recovery
	_ = os.Remove(eclassFile)
	_, err = rule.getEclassMD5(eclassFile)
	if err == nil {
		t.Fatalf("Expected error for missing file")
	}

	content4 := []byte("v4")
	_ = os.WriteFile(eclassFile, content4, 0644)
	_ = os.Chtimes(eclassFile, baseTime, baseTime)
	hash4, err := rule.getEclassMD5(eclassFile)
	if err != nil || hash4 != fmt.Sprintf("%x", md5.Sum(content4)) {
		t.Fatalf("Failed recovery invalidation")
	}
}

func TestMD5CacheInvalidLintRule_ReuseAndIsolation(t *testing.T) {
	rule := &MD5CacheInvalidLintRule{}
	tempDir := t.TempDir()

	// Repo 1
	repo1 := filepath.Join(tempDir, "repo1")
	eclassDir1 := filepath.Join(repo1, "eclass")
	_ = os.MkdirAll(eclassDir1, 0755)
	eclassFile1 := filepath.Join(eclassDir1, "test.eclass")
	_ = os.WriteFile(eclassFile1, []byte("repo1_content"), 0644)

	// Repo 2 (different content, same name)
	repo2 := filepath.Join(tempDir, "repo2")
	eclassDir2 := filepath.Join(repo2, "eclass")
	_ = os.MkdirAll(eclassDir2, 0755)
	eclassFile2 := filepath.Join(eclassDir2, "test.eclass")
	_ = os.WriteFile(eclassFile2, []byte("repo2_content"), 0644)

	hash1, _ := rule.getEclassMD5(eclassFile1)
	hash2, _ := rule.getEclassMD5(eclassFile2)

	if hash1 == hash2 {
		t.Fatalf("Cache isolation failed")
	}

	// Test Symlink / Shared
	repo3 := filepath.Join(tempDir, "repo3")
	eclassDir3 := filepath.Join(repo3, "eclass")
	_ = os.MkdirAll(eclassDir3, 0755)
	eclassFile3 := filepath.Join(eclassDir3, "test.eclass")

	err := os.Symlink(eclassFile1, eclassFile3)
	if err == nil {
		hash3, _ := rule.getEclassMD5(eclassFile3)
		if hash3 != hash1 {
			t.Fatalf("Shared symlink caching failed")
		}
	} else {
		t.Logf("Symlink not supported, skipping shared test: %v", err)
	}
}

func TestMD5CacheInvalidLintRule_Eviction(t *testing.T) {
	rule := &MD5CacheInvalidLintRule{limit: 2}
	tempDir := t.TempDir()

	eclassDir := filepath.Join(tempDir, "eclass")
	_ = os.MkdirAll(eclassDir, 0755)

	files := []string{"a.eclass", "b.eclass", "c.eclass"}
	for _, f := range files {
		p := filepath.Join(eclassDir, f)
		_ = os.WriteFile(p, []byte(f), 0644)
	}

	// load a, b
	_, _ = rule.getEclassMD5(filepath.Join(eclassDir, "a.eclass"))
	_, _ = rule.getEclassMD5(filepath.Join(eclassDir, "b.eclass"))

	rule.mu.Lock()
	l := rule.lru.Len()
	rule.mu.Unlock()
	if l != 2 {
		t.Fatalf("Expected 2 items, got %d", l)
	}

	// load c, should evict a
	_, _ = rule.getEclassMD5(filepath.Join(eclassDir, "c.eclass"))
	rule.mu.Lock()
	l = rule.lru.Len()
	rule.mu.Unlock()
	if l != 2 {
		t.Fatalf("Expected 2 items after eviction, got %d", l)
	}

	// Check if a is evicted
	rule.mu.Lock()
	_, ok := rule.cache[canonicalEclassPath(filepath.Join(eclassDir, "a.eclass"))]
	rule.mu.Unlock()
	if ok {
		t.Fatalf("Expected a.eclass to be evicted")
	}
}

func TestMD5CacheInvalidLintRule_Concurrency(t *testing.T) {
	rule := &MD5CacheInvalidLintRule{}
	tempDir := t.TempDir()

	eclassDir := filepath.Join(tempDir, "eclass")
	_ = os.MkdirAll(eclassDir, 0755)

	var files []string
	for i := 0; i < 5; i++ {
		p := filepath.Join(eclassDir, fmt.Sprintf("test%d.eclass", i))
		_ = os.WriteFile(p, []byte(fmt.Sprintf("content%d", i)), 0644)
		files = append(files, p)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			file := files[idx%len(files)]
			_, err := rule.getEclassMD5(file)
			if err != nil {
				t.Errorf("Concurrent get failed: %v", err)
			}
		}(i)
	}
	wg.Wait()
}
