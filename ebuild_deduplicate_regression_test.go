package g2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestEbuild(t *testing.T, dir, name, content string) {
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
	if err != nil {
		t.Fatalf("writing ebuild %s: %v", name, err)
	}
}

func writeTestManifest(t *testing.T, dir, content string) {
	err := os.WriteFile(filepath.Join(dir, "Manifest"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("writing manifest: %v", err)
	}
}

func TestDeduplicateEbuildsRegression(t *testing.T) {
	dir := t.TempDir()

	// Create two identical digest ebuilds
	// One is older
	writeTestEbuild(t, dir, "foo-1.0.ebuild", `EAPI=8
DESCRIPTION="foo"
SLOT="0"
SRC_URI="https://example.com/good.tar.gz"
`)

	// One is newer, but relies on eclass (no SRC_URI but inherits eclass)
	writeTestEbuild(t, dir, "foo-1.0-r1.ebuild", `EAPI=8
inherit pypi
DESCRIPTION="foo"
SLOT="0"
`)

	// The manifest has the good.tar.gz file
	writeTestManifest(t, dir, "DIST good.tar.gz 123 SHA512 abc\n")

	// Run DeduplicateEbuilds
	removed, err := DeduplicateEbuilds([]string{dir})
	if err != nil {
		t.Fatalf("DeduplicateEbuilds failed: %v", err)
	}

	if len(removed) != 1 || filepath.Base(removed[0]) != "foo-1.0.ebuild" {
		t.Fatalf("Expected foo-1.0.ebuild to be removed, removed: %v", removed)
	}

	// Verify Manifest was preserved because foo-1.0-r1 is uncertain
	b, err := os.ReadFile(filepath.Join(dir, "Manifest"))
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}
	result := string(b)
	if !strings.Contains(result, "DIST good.tar.gz") {
		t.Errorf("Expected good.tar.gz to be preserved in manifest, got: %s", result)
	}
}
