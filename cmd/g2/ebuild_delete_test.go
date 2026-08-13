package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCmdEbuildDelete(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup a fake repo
	pkgDir := filepath.Join(tmpDir, "app-test", "test-pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create pkg dir: %v", err)
	}

	ebuildPath := filepath.Join(pkgDir, "test-pkg-1.0.ebuild")
	if err := os.WriteFile(ebuildPath, []byte("EAPI=8\n"), 0644); err != nil {
		t.Fatalf("Failed to write ebuild: %v", err)
	}

	manifestPath := filepath.Join(pkgDir, "Manifest")
	if err := os.WriteFile(manifestPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}

	metadataPath := filepath.Join(pkgDir, "metadata.xml")
	if err := os.WriteFile(metadataPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write metadata: %v", err)
	}

	cfg := &CmdEbuildArgConfig{
		MainArgConfig: &MainArgConfig{},
	}

	// Delete by path
	if err := cfg.cmdEbuildDelete([]string{"--repo", tmpDir, ebuildPath}); err != nil {
		t.Fatalf("cmdEbuildDelete failed: %v", err)
	}

	if _, err := os.Stat(ebuildPath); !os.IsNotExist(err) {
		t.Errorf("Ebuild was not deleted")
	}
	if _, err := os.Stat(pkgDir); !os.IsNotExist(err) {
		t.Errorf("Package directory was not completely deleted")
	}

	// Test by atom
	pkgDir = filepath.Join(tmpDir, "app-test", "test-pkg2")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create pkg dir: %v", err)
	}

	ebuildPath = filepath.Join(pkgDir, "test-pkg2-1.0.ebuild")
	if err := os.WriteFile(ebuildPath, []byte("EAPI=8\n"), 0644); err != nil {
		t.Fatalf("Failed to write ebuild: %v", err)
	}

	if err := cfg.cmdEbuildDelete([]string{"--repo", tmpDir, "app-test/test-pkg2-1.0"}); err != nil {
		t.Fatalf("cmdEbuildDelete failed: %v", err)
	}

	if _, err := os.Stat(ebuildPath); !os.IsNotExist(err) {
		t.Errorf("Ebuild was not deleted by atom")
	}
}
