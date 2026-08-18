package layout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arran4/g2"
)

func TestRepoLayoutLintRule(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "g2-layout-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create valid directories
	os.MkdirAll(filepath.Join(tempDir, "profiles"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "metadata"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "eclass"), 0755)

	// Create categories file
	os.WriteFile(filepath.Join(tempDir, "profiles", "categories"), []byte("app-test\n"), 0644)
	os.MkdirAll(filepath.Join(tempDir, "app-test"), 0755)

	// Create stray directory and file
	os.MkdirAll(filepath.Join(tempDir, "stray-dir"), 0755)
	os.WriteFile(filepath.Join(tempDir, "stray-file.txt"), []byte("test"), 0644)

	// Create a .g2ignore file
	os.WriteFile(filepath.Join(tempDir, ".g2ignore"), []byte("stray-dir\nignored-file.txt\n"), 0644)
	os.WriteFile(filepath.Join(tempDir, "ignored-file.txt"), []byte("test"), 0644)

	// Set global config
	LayoutLintEnabled = true
	AllowGithubAPI = false
	UpstreamRepoPath = ""

	rule := &RepoLayoutLintRule{}
	results := rule.LintRepo(tempDir, &g2.SiteData{})

	// Expect exactly 1 error for stray-file.txt
	if len(results) != 1 {
		t.Fatalf("expected 1 lint error, got %d. Results: %v", len(results), results)
	}

	if !strings.Contains(results[0].Message, "stray-file.txt") {
		t.Errorf("expected error about stray-file.txt, got: %s", results[0].Message)
	}
}
