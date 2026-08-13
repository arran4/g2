package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestCmdEbuildDelete(t *testing.T) {
	mockFS := NewMockFS()

	// Setup a fake repo
	pkgDir := "app-test/test-pkg"
	mockFS.MkdirAll(pkgDir, 0755)

	ebuildPath := "app-test/test-pkg/test-pkg-1.0.ebuild"
	mockFS.WriteFile(ebuildPath, []byte("EAPI=8\n"), 0644)

	manifestPath := "app-test/test-pkg/Manifest"
	mockFS.WriteFile(manifestPath, []byte(""), 0644)

	metadataPath := "app-test/test-pkg/metadata.xml"
	mockFS.WriteFile(metadataPath, []byte(""), 0644)

	// Create absolute path versions for MockFS because DeleteEbuilds converts to absolute
	absEbuildPath, _ := filepath.Abs(ebuildPath)
	mockFS.WriteFile(absEbuildPath, []byte("EAPI=8\n"), 0644)
	absPkgDir := filepath.Dir(absEbuildPath)
	mockFS.WriteFile(filepath.Join(absPkgDir, "Manifest"), []byte(""), 0644)
	mockFS.WriteFile(filepath.Join(absPkgDir, "metadata.xml"), []byte(""), 0644)

	cfg := &CmdEbuildArgConfig{
		MainArgConfig: &MainArgConfig{},
	}

	// Delete by path
	if err := cfg.DeleteEbuilds(mockFS, []string{ebuildPath}, "."); err != nil {
		t.Fatalf("DeleteEbuilds failed: %v", err)
	}

	if _, err := fs.Stat(mockFS, absEbuildPath); !os.IsNotExist(err) {
		t.Errorf("Ebuild was not deleted")
	}
	if _, err := fs.Stat(mockFS, absPkgDir); !os.IsNotExist(err) {
		t.Errorf("Package directory was not completely deleted")
	}

	// Test by atom
	pkgDir2 := "app-test/test-pkg2"
	mockFS.MkdirAll(pkgDir2, 0755)

	ebuildPath2 := "app-test/test-pkg2/test-pkg2-1.0.ebuild"
	mockFS.WriteFile(ebuildPath2, []byte("EAPI=8\n"), 0644)

	if err := cfg.DeleteEbuilds(mockFS, []string{"app-test/test-pkg2-1.0"}, "."); err != nil {
		t.Fatalf("DeleteEbuilds failed: %v", err)
	}

	if _, err := fs.Stat(mockFS, ebuildPath2); !os.IsNotExist(err) {
		t.Errorf("Ebuild was not deleted by atom")
	}
}
