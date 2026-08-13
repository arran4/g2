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
	if err := mockFS.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	ebuildPath := "app-test/test-pkg/test-pkg-1.0.ebuild"
	if err := mockFS.WriteFile(ebuildPath, []byte("EAPI=8\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	manifestPath := "app-test/test-pkg/Manifest"
	if err := mockFS.WriteFile(manifestPath, []byte(""), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	metadataPath := "app-test/test-pkg/metadata.xml"
	if err := mockFS.WriteFile(metadataPath, []byte(""), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Create absolute path versions for MockFS because DeleteEbuilds converts to absolute
	absEbuildPath, _ := filepath.Abs(ebuildPath)
	if err := mockFS.WriteFile(absEbuildPath, []byte("EAPI=8\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	absPkgDir := filepath.Dir(absEbuildPath)
	if err := mockFS.WriteFile(filepath.Join(absPkgDir, "Manifest"), []byte(""), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := mockFS.WriteFile(filepath.Join(absPkgDir, "metadata.xml"), []byte(""), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

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
	if err := mockFS.MkdirAll(pkgDir2, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	ebuildPath2 := "app-test/test-pkg2/test-pkg2-1.0.ebuild"
	if err := mockFS.WriteFile(ebuildPath2, []byte("EAPI=8\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := cfg.DeleteEbuilds(mockFS, []string{"app-test/test-pkg2-1.0"}, "."); err != nil {
		t.Fatalf("DeleteEbuilds failed: %v", err)
	}

	if _, err := fs.Stat(mockFS, ebuildPath2); !os.IsNotExist(err) {
		t.Errorf("Ebuild was not deleted by atom")
	}
}
