package ebuild

import (
	"path/filepath"
	"testing"

	"github.com/arran4/g2"
	"testing/fstest"
)

func TestMissingManifestLintRule(t *testing.T) {
	mockFS := fstest.MapFS{
		filepath.Join("app-test", "testpkg", "testpkg-1.0.ebuild"): &fstest.MapFile{
			Data: []byte(`SRC_URI="https://example.com/file1.tar.gz"`),
		},
		filepath.Join("app-test", "testpkg", "testpkg-2.0.ebuild"): &fstest.MapFile{
			Data: []byte(`SRC_URI="https://example.com/file2.tar.gz"`),
		},
	}

	category := "app-test"
	name := "testpkg"

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
	results := rule.lintWithFS(mockFS, filepath.Join(category, name), pkgData, nil)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}

	expectedMsg := "[Error] version 2.0: distfile missing from Manifest: [ file2.tar.gz ]"
	if results[0].Message != expectedMsg {
		t.Errorf("expected message '%s', got '%s'", expectedMsg, results[0].Message)
	}
	if results[0].File != "testpkg-2.0.ebuild" {
		t.Errorf("expected file 'testpkg-2.0.ebuild', got '%s'", results[0].File)
	}
}
