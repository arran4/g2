package ebuild

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arran4/g2"
)

func TestMissingManifestLintRule(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "missing-manifest-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	category := "app-test"
	name := "testpkg"
	pkgDir := filepath.Join(tempDir, category, name)

	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	ebuildContent1 := `SRC_URI="https://example.com/file1.tar.gz"`
	if err := os.WriteFile(filepath.Join(pkgDir, "testpkg-1.0.ebuild"), []byte(ebuildContent1), 0644); err != nil {
		t.Fatalf("failed to write ebuild: %v", err)
	}

	ebuildContent2 := `SRC_URI="https://example.com/file2.tar.gz"`
	if err := os.WriteFile(filepath.Join(pkgDir, "testpkg-2.0.ebuild"), []byte(ebuildContent2), 0644); err != nil {
		t.Fatalf("failed to write ebuild: %v", err)
	}

	pkgData := &g2.PackageData{
		Category: category,
		Name:     name,
		Manifest: &g2.Manifest{
			Entries: []*g2.ManifestEntry{
				{
					Type:     "DIST",
					Filename: "file1.tar.gz",
				},
			},
		},
	}

	rule := &MissingManifestLintRule{}
	results := rule.Lint(tempDir, pkgData)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	expectedMsg := "[Error] version 2.0: distfile missing from Manifest: [ file2.tar.gz ]"
	if results[0].Message != expectedMsg {
		t.Errorf("expected message '%s', got '%s'", expectedMsg, results[0].Message)
	}
	if results[0].File != "testpkg-2.0.ebuild" {
		t.Errorf("expected file 'testpkg-2.0.ebuild', got '%s'", results[0].File)
	}
}
