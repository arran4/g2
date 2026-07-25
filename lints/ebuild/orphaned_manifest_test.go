package ebuild_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestOrphanedManifestLintRule(t *testing.T) {
	rule := &ebuild.OrphanedManifestLintRule{}

	// Create a temporary directory structure for the test repo
	repoDir := t.TempDir()
	pkgDir := filepath.Join(repoDir, "app-misc", "testpkg")
	if err := os.MkdirAll(filepath.Join(pkgDir, "files"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create an ebuild that uses a DIST file
	ebuildPath := filepath.Join(pkgDir, "testpkg-1.0.ebuild")
	ebuildContent := `SRC_URI="https://example.com/file1.tar.gz"`
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a used AUX file
	if err := os.WriteFile(filepath.Join(pkgDir, "files", "used.patch"), []byte("patch content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Set up the package data
	pkg := &g2.PackageData{
		Category: "app-misc",
		Name:     "testpkg",
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				Ebuild: &g2.Ebuild{
					Vars: map[string]string{
						"SRC_URI": "https://example.com/file1.tar.gz",
					},
				},
			},
		},
		Manifest: &g2.Manifest{
			Entries: []*g2.ManifestEntry{
				// Used DIST file (should not error)
				{Type: "DIST", Filename: "file1.tar.gz"},
				// Unused DIST file (should error)
				{Type: "DIST", Filename: "file2.tar.gz"},
				// Used AUX file (should not error)
				{Type: "AUX", Filename: "used.patch"},
				// Non-existent AUX file (should error)
				{Type: "AUX", Filename: "missing.patch"},
			},
		},
	}

	warnings := rule.Lint(repoDir, pkg)

	expectedWarnings := []string{
		"Manifest entry for unused DIST file 'file2.tar.gz'",
		"Manifest entry for non-existent AUX file 'missing.patch'",
	}

	if len(warnings) != len(expectedWarnings) {
		t.Errorf("Expected %d warnings, got %d:\n%v", len(expectedWarnings), len(warnings), warnings)
	}

	for _, expected := range expectedWarnings {
		found := false
		for _, warn := range warnings {
			if strings.Contains(warn.Message, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected warning containing: %s\nGot warnings:\n%v", expected, warnings)
		}
	}
}
