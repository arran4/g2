package md5cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

func TestMD5CacheMissing(t *testing.T) {
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

	rule := &MD5CacheLintRule{}

	// Test case 1: md5-cache directory does not exist, default QA policy
	// Expect no results because rule should be dynamically disabled.
	results := rule.LintWithQA(tempDir, pkg, nil)
	if len(results) != 0 {
		t.Errorf("Expected 0 results when md5-cache dir is missing, got %d", len(results))
	}

	// Test case 2: md5-cache directory does not exist, but explicitly enforced via QA policy
	qaPolicy := &g2.QAPolicy{
		Policies: map[string]string{
			"PG0802": "error",
		},
	}
	results = rule.LintWithQA(tempDir, pkg, qaPolicy)
	if len(results) != 1 {
		t.Errorf("Expected 1 result when explicitly enforced, got %d", len(results))
	} else if results[0].RuleMetadata.Severity != lints.SeverityError {
		t.Errorf("Expected severity to be error, got %v", results[0].RuleMetadata.Severity)
	}

	// Test case 3: md5-cache directory exists, but cache for package is missing
	md5CacheDir := filepath.Join(tempDir, "metadata", "md5-cache")
	if err := os.MkdirAll(md5CacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	results = rule.LintWithQA(tempDir, pkg, nil)
	if len(results) != 1 {
		t.Errorf("Expected 1 result when md5-cache dir exists but file is missing, got %d", len(results))
	} else if results[0].RuleMetadata.Severity != lints.SeverityWarning {
		t.Errorf("Expected severity to be warning, got %v", results[0].RuleMetadata.Severity)
	}

	// Test case 4: cache file exists
	pkgCacheDir := filepath.Join(md5CacheDir, "app-misc")
	if err := os.MkdirAll(pkgCacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	cacheFile := filepath.Join(pkgCacheDir, "foo-1.0")
	if err := os.WriteFile(cacheFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	results = rule.LintWithQA(tempDir, pkg, nil)
	if len(results) != 0 {
		t.Errorf("Expected 0 results when cache file exists, got %d", len(results))
	}
}
