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

	writeTestEbuild(t, dir, "foo-1.0.ebuild", "EAPI=8\nDESCRIPTION=\"foo\"\nSLOT=\"0\"\nSRC_URI=\"https://example.com/good.tar.gz\"\n")
	writeTestEbuild(t, dir, "foo-1.0-r1.ebuild", "EAPI=8\nif true; then inherit pypi; fi\nDESCRIPTION=\"foo\"\nSLOT=\"0\"\n")

	writeTestManifest(t, dir, "DIST good.tar.gz 123 SHA512 abc\n")

	removed, err := DeduplicateEbuilds([]string{dir})
	if err != nil {
		t.Fatalf("DeduplicateEbuilds failed: %v", err)
	}

	if len(removed) != 1 || filepath.Base(removed[0]) != "foo-1.0.ebuild" {
		t.Fatalf("Expected foo-1.0.ebuild to be removed, removed: %v", removed)
	}

	b, err := os.ReadFile(filepath.Join(dir, "Manifest"))
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}
	result := string(b)
	if !strings.Contains(result, "DIST good.tar.gz") {
		t.Errorf("Expected good.tar.gz to be preserved in manifest, got: %s", result)
	}
}

func TestDeduplicateEbuildsWriteErrorRegression(t *testing.T) {
	dir := t.TempDir()

	writeTestEbuild(t, dir, "foo-1.0.ebuild", `EAPI=8
DESCRIPTION="foo"
SLOT="0"
SRC_URI="https://example.com/good.tar.gz"
`)

	writeTestEbuild(t, dir, "foo-2.0.ebuild", `EAPI=8
DESCRIPTION="foo"
SLOT="0"
SRC_URI="https://example.com/good.tar.gz"
`)

	writeTestManifest(t, dir, "DIST good.tar.gz 123 SHA512 abc\n")

	// Make writing fail
	_ = os.Remove(filepath.Join(dir, "Manifest"))
	_ = os.Mkdir(filepath.Join(dir, "Manifest"), 0755)

	removed, err := DeduplicateEbuilds([]string{dir})
	if err == nil {
		t.Fatalf("DeduplicateEbuilds expected to fail due to error")
	}

	if len(removed) != 1 || filepath.Base(removed[0]) != "foo-1.0.ebuild" {
		t.Fatalf("Expected foo-1.0.ebuild to be removed before failure, got: %v", removed)
	}
}
