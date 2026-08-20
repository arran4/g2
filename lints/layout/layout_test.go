package layout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arran4/g2"
)

func TestRepoLayoutLintRule(t *testing.T) {
	tempDir := t.TempDir()

	// Create valid directories
	if err := os.MkdirAll(filepath.Join(tempDir, "profiles"), 0755); err != nil {
		t.Fatalf("failed to create profiles dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tempDir, "metadata"), 0755); err != nil {
		t.Fatalf("failed to create metadata dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tempDir, "eclass"), 0755); err != nil {
		t.Fatalf("failed to create eclass dir: %v", err)
	}

	// Create categories file
	if err := os.WriteFile(filepath.Join(tempDir, "profiles", "categories"), []byte("app-test\n"), 0644); err != nil {
		t.Fatalf("failed to write categories file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tempDir, "app-test"), 0755); err != nil {
		t.Fatalf("failed to create category dir: %v", err)
	}

	// Create stray directory and file
	if err := os.MkdirAll(filepath.Join(tempDir, "stray-dir"), 0755); err != nil {
		t.Fatalf("failed to create stray dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "stray-file.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write stray file: %v", err)
	}

	// Create a .g2ignore file
	if err := os.WriteFile(filepath.Join(tempDir, ".g2ignore"), []byte("stray-dir\nignored-file.txt\n"), 0644); err != nil {
		t.Fatalf("failed to write .g2ignore file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "ignored-file.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write ignored file: %v", err)
	}

	// Set global config
	LayoutLintEnabled = true
	AllowGithubAPI = false
	UpstreamRepoPath = ""

	rule := &RepoLayoutLintRule{}
	results := rule.LintRepo(tempDir, &g2.SiteData{}, nil)

	// Expect exactly 1 error for stray-file.txt
	if len(results) != 1 {
		t.Fatalf("expected 1 lint error, got %d. Results: %v", len(results), results)
	}

	if !strings.Contains(results[0].Message, "stray-file.txt") {
		t.Errorf("expected error about stray-file.txt, got: %s", results[0].Message)
	}
}
