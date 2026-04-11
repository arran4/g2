package metadata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

func TestManifestLintRule_MissingFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "manifest-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	category := "app-test"
	name := "testpkg"

	pkgDir := filepath.Join(tempDir, category, name)
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	// Create a layout.conf that requires BLAKE2B
	metadataDir := filepath.Join(tempDir, "metadata")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		t.Fatalf("failed to create metadata dir: %v", err)
	}
	layoutConf := "masters = gentoo\nmanifest-required-hashes = BLAKE2B\n"
	if err := os.WriteFile(filepath.Join(metadataDir, "layout.conf"), []byte(layoutConf), 0644); err != nil {
		t.Fatalf("failed to write layout.conf: %v", err)
	}

	manifest := &g2.Manifest{
		Entries: []*g2.ManifestEntry{
			{
				Type:     "EBUILD",
				Filename: "testpkg-1.0.ebuild",
				Size:     100,
				Hashes: []g2.Hash{
					{Type: "BLAKE2B", Value: "hash1"},
				},
			},
			{
				Type:     "MISC",
				Filename: "metadata.xml",
				Size:     100,
				Hashes: []g2.Hash{
					{Type: "BLAKE2B", Value: "hash2"},
				},
			},
			{
				Type:     "AUX",
				Filename: "test.patch",
				Size:     100,
				Hashes: []g2.Hash{
					{Type: "BLAKE2B", Value: "hash3"},
				},
			},
		},
	}

	pkgData := &g2.PackageData{
		Category: category,
		Name:     name,
		Manifest: manifest,
	}

	rule := &ManifestLintRule{}
	results := rule.Lint(tempDir, pkgData)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	for _, result := range results {
		if result.RuleMetadata.Severity != lints.SeverityError {
			t.Errorf("expected error severity, got %v", result.RuleMetadata.Severity)
		}
		if result.RuleMetadata.ID != ruleManifestChecks.ID {
			t.Errorf("expected rule ID %s, got %s", ruleManifestChecks.ID, result.RuleMetadata.ID)
		}
	}

	// Now create the files and ensure no errors are reported
	if err := os.WriteFile(filepath.Join(pkgDir, "testpkg-1.0.ebuild"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to write ebuild: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "metadata.xml"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to write metadata.xml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(pkgDir, "files"), 0755); err != nil {
		t.Fatalf("failed to create files dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "files", "test.patch"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to write patch: %v", err)
	}

	results = rule.Lint(tempDir, pkgData)
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d: %v", len(results), results)
	}
}
