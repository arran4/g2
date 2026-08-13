package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCmdEbuildInstall(t *testing.T) {
	tmpDir := t.TempDir()

	ebuildContent := []byte(`EAPI=8
CATEGORY="app-test"
DESCRIPTION="Test"
`)
	ebuildFile := filepath.Join(tmpDir, "test-pkg-1.0.ebuild")
	if err := os.WriteFile(ebuildFile, ebuildContent, 0644); err != nil {
		t.Fatalf("Failed to write ebuild: %v", err)
	}

	repoDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	cfg := &CmdEbuildArgConfig{
		MainArgConfig: &MainArgConfig{},
	}

	// Install file
	if err := cfg.cmdEbuildInstall([]string{"--repo", repoDir, ebuildFile}); err != nil {
		t.Fatalf("cmdEbuildInstall failed: %v", err)
	}

	targetFile := filepath.Join(repoDir, "app-test", "test-pkg", "test-pkg-1.0.ebuild")
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Errorf("Ebuild was not installed to expected path: %s", targetFile)
	}
}
