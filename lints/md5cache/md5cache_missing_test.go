package md5cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

type mockFileStat struct {
	paths map[string]bool
}

func (m mockFileStat) Stat(name string) (os.FileInfo, error) {
	if _, ok := m.paths[name]; ok {
		return nil, nil // Return a fake file info, not used by code anyway.
	}
	return nil, os.ErrNotExist
}

func TestMD5CacheMissing(t *testing.T) {
	repoDir := "/mock/repo"

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

	// Test case 1: md5-cache directory does not exist, default QA policy
	rule := &MD5CacheLintRule{fs: mockFileStat{paths: map[string]bool{}}}
	results := rule.LintWithQA(repoDir, pkg, nil)
	if len(results) != 0 {
		t.Errorf("Expected 0 results when md5-cache dir is missing, got %d", len(results))
	}

	// Test case 2: md5-cache directory does not exist, but explicitly enforced via QA policy
	qaPolicy := &g2.QAPolicy{
		Policies: map[string]string{
			"PG0802": "error",
		},
	}
	results = rule.LintWithQA(repoDir, pkg, qaPolicy)
	if len(results) != 1 {
		t.Errorf("Expected 1 result when explicitly enforced, got %d", len(results))
	} else if results[0].RuleMetadata.Severity != lints.SeverityError {
		t.Errorf("Expected severity to be error, got %v", results[0].RuleMetadata.Severity)
	}

	// Test case 3: md5-cache directory exists, but cache for package is missing
	rule = &MD5CacheLintRule{fs: mockFileStat{paths: map[string]bool{
		filepath.Join(repoDir, "metadata", "md5-cache"): true,
	}}}
	results = rule.LintWithQA(repoDir, pkg, nil)
	if len(results) != 1 {
		t.Errorf("Expected 1 result when md5-cache dir exists but file is missing, got %d", len(results))
	} else if results[0].RuleMetadata.Severity != lints.SeverityWarning {
		t.Errorf("Expected severity to be warning, got %v", results[0].RuleMetadata.Severity)
	}

	// Test case 4: cache file exists
	rule = &MD5CacheLintRule{fs: mockFileStat{paths: map[string]bool{
		filepath.Join(repoDir, "metadata", "md5-cache"):                        true,
		filepath.Join(repoDir, "metadata", "md5-cache", "app-misc", "foo-1.0"): true,
	}}}
	results = rule.LintWithQA(repoDir, pkg, nil)
	if len(results) != 0 {
		t.Errorf("Expected 0 results when cache file exists, got %d", len(results))
	}
}
