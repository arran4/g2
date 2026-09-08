package g2

import (
	"os"
	"path/filepath"
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

	originalManifest := "# Comments\n\nDIST good.tar.gz 123 SHA512 abc  \n"
	writeTestManifest(t, dir, originalManifest)

	removed, err := DeduplicateEbuilds([]string{dir})
	if err != nil {
		t.Fatalf("DeduplicateEbuilds failed: %v", err)
	}

	if len(removed) != 1 || filepath.Base(removed[0]) != "foo-1.0.ebuild" {
		t.Fatalf("Expected foo-1.0.ebuild to be removed, removed: %v", removed)
	}

	if _, err := os.Stat(filepath.Join(dir, "foo-1.0-r1.ebuild")); err != nil {
		t.Fatalf("Expected foo-1.0-r1.ebuild to be retained")
	}

	b, err := os.ReadFile(filepath.Join(dir, "Manifest"))
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}
	result := string(b)
	if result != originalManifest {
		t.Errorf("Expected exactly preserved manifest bytes, got:\n%q\nwant:\n%q", result, originalManifest)
	}
}


func TestDeduplicateEbuildsWriteErrorRegression(t *testing.T) {
	dir := t.TempDir()

	writeTestEbuild(t, dir, "foo-1.0.ebuild", "EAPI=8\nDESCRIPTION=\"foo\"\nSLOT=\"0\"\nSRC_URI=\"https://example.com/good.tar.gz\"\n")
	writeTestEbuild(t, dir, "foo-2.0.ebuild", "EAPI=8\nDESCRIPTION=\"foo\"\nSLOT=\"0\"\nSRC_URI=\"https://example.com/good.tar.gz\"\n")
	writeTestManifest(t, dir, "DIST good.tar.gz 123 SHA512 abc\n")

	atomicWriteManifestTestHook = func(path string, m *Manifest) error {
		return os.ErrPermission
	}
	t.Cleanup(func() { atomicWriteManifestTestHook = nil })

	removed, err := DeduplicateEbuilds([]string{dir})
	if err == nil {
		t.Fatalf("DeduplicateEbuilds expected to fail due to write error")
	}

	if len(removed) != 1 || filepath.Base(removed[0]) != "foo-1.0.ebuild" {
		t.Fatalf("Expected foo-1.0.ebuild to be removed before failure, got: %v", removed)
	}
}

func TestAtomicWriteManifestFailure(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "Manifest")
	m := &Manifest{}

	_ = os.Mkdir(manifestPath, 0755)

	err := AtomicWriteManifest(manifestPath, m)
	if err == nil {
		t.Fatalf("Expected AtomicWriteManifest to fail on directory rename")
	}
}

func TestAtomicWriteManifestTempFile(t *testing.T) {
	t.Run("Direct temp-path regression for AtomicWriteManifest", func(t *testing.T) {
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "Manifest")
		m := &Manifest{}

		tmpPath := filepath.Join(dir, "Manifest.preexisting.tmp")
		_ = os.WriteFile(tmpPath, []byte("garbage data"), 0644)

		err := AtomicWriteManifest(manifestPath, m)
		if err != nil {
			t.Fatalf("Expected AtomicWriteManifest to succeed, got %v", err)
		}

		b, err := os.ReadFile(tmpPath)
		if err != nil || string(b) != "garbage data" {
			t.Errorf("Pre-existing similar temp file clobbered or removed! Bytes: %q, err: %v", string(b), err)
		}
	})
}
