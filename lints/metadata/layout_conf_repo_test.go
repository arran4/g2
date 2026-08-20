package metadata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arran4/g2/lints"
)

func TestLayoutConfRepoLintRule(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "layout-conf-repo-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	metadataDir := filepath.Join(tempDir, "metadata")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		t.Fatalf("failed to create metadata dir: %v", err)
	}

	rule := &LayoutConfRepoLintRule{}

	// Test 1: Missing layout.conf
	results := rule.LintRepo(tempDir, nil)
	if len(results) != 1 {
		t.Fatalf("Test 1: expected 1 result, got %d", len(results))
	}
	if results[0].RuleMetadata.Severity != lints.SeverityError {
		t.Errorf("Test 1: expected error severity")
	}

	// Test 2: Missing masters
	layoutConf := "sign-commits = true\n"
	if err := os.WriteFile(filepath.Join(metadataDir, "layout.conf"), []byte(layoutConf), 0644); err != nil {
		t.Fatalf("failed to write layout.conf: %v", err)
	}
	results = rule.LintRepo(tempDir, nil)
	if len(results) != 1 {
		t.Fatalf("Test 2: expected 1 result, got %d", len(results))
	}
	if results[0].RuleMetadata.Severity != lints.SeverityError {
		t.Errorf("Test 2: expected error severity")
	}

	// Test 3: Invalid use-manifests
	layoutConf = "masters = gentoo\nuse-manifests = maybe\n"
	if err := os.WriteFile(filepath.Join(metadataDir, "layout.conf"), []byte(layoutConf), 0644); err != nil {
		t.Fatalf("failed to write layout.conf: %v", err)
	}
	results = rule.LintRepo(tempDir, nil)
	if len(results) != 1 {
		t.Fatalf("Test 3: expected 1 result, got %d", len(results))
	}
	if results[0].RuleMetadata.Severity != lints.SeverityError {
		t.Errorf("Test 3: expected error severity")
	}

	// Test 4: Invalid boolean values
	layoutConf = "masters = gentoo\nthin-manifests = yes\nsign-commits = false\n"
	if err := os.WriteFile(filepath.Join(metadataDir, "layout.conf"), []byte(layoutConf), 0644); err != nil {
		t.Fatalf("failed to write layout.conf: %v", err)
	}
	results = rule.LintRepo(tempDir, nil)
	if len(results) != 1 {
		t.Fatalf("Test 4: expected 1 result, got %d", len(results))
	}
	if results[0].RuleMetadata.Severity != lints.SeverityError {
		t.Errorf("Test 4: expected error severity")
	}

	// Test 5: Unknown keys
	layoutConf = "masters = gentoo\nunknown-key = something\n"
	if err := os.WriteFile(filepath.Join(metadataDir, "layout.conf"), []byte(layoutConf), 0644); err != nil {
		t.Fatalf("failed to write layout.conf: %v", err)
	}
	results = rule.LintRepo(tempDir, nil)
	if len(results) != 1 {
		t.Fatalf("Test 5: expected 1 result, got %d", len(results))
	}
	if results[0].RuleMetadata.Severity != lints.SeverityNotice {
		t.Errorf("Test 5: expected notice severity")
	}

	// Test 6: Valid layout.conf
	layoutConf = "masters = gentoo\nuse-manifests = strict\nthin-manifests = true\n"
	if err := os.WriteFile(filepath.Join(metadataDir, "layout.conf"), []byte(layoutConf), 0644); err != nil {
		t.Fatalf("failed to write layout.conf: %v", err)
	}
	results = rule.LintRepo(tempDir, nil)
	if len(results) != 0 {
		t.Fatalf("Test 6: expected 0 results, got %d", len(results))
	}
}
