package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEbuildBumpVersion(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "ebuild-bump-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a fake ebuild
	pn := "testpkg"
	oldPv := "1.0.0"
	oldEbuildName := pn + "-" + oldPv + ".ebuild"
	oldEbuildPath := filepath.Join(tmpDir, oldEbuildName)

	// Create some dummy ebuild content.
	ebuildContent := `
EAPI=8
DESCRIPTION="A test package"
SRC_URI="https://example.com/${P}.tar.gz"
`
	if err := os.WriteFile(oldEbuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatalf("Failed to create fake ebuild: %v", err)
	}

	// Create a dummy manifest
	manifestPath := filepath.Join(tmpDir, "Manifest")
	manifestContent := `DIST testpkg-1.0.0.tar.gz 1024 BLAKE2B 1234567890abcdef SHA512 1234567890abcdef
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("Failed to create dummy manifest: %v", err)
	}

	// Run the bump-version command
	cfg := &CmdEbuildArgConfig{}
	newPv := "1.1.0"

	args := []string{oldEbuildPath, newPv}
	if err := cfg.cmdEbuildBumpVersion(args); err != nil {
		t.Fatalf("cmdEbuildBumpVersion failed: %v", err)
	}

	// Check if old ebuild is gone
	if _, err := os.Stat(oldEbuildPath); !os.IsNotExist(err) {
		t.Errorf("Old ebuild file %s was not removed/renamed", oldEbuildPath)
	}

	// Check if new ebuild exists
	newEbuildPath := filepath.Join(tmpDir, pn+"-"+newPv+".ebuild")
	if _, err := os.Stat(newEbuildPath); err != nil {
		t.Errorf("New ebuild file %s was not created: %v", newEbuildPath, err)
	}

	// Check if manifest is updated properly
	newManifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("Failed to read manifest: %v", err)
	}

	newManifestContent := string(newManifestBytes)

	// Old entry should be removed
	if strings.Contains(newManifestContent, "testpkg-1.0.0.tar.gz") {
		t.Errorf("Manifest still contains old entry")
	}

	// Note: in a real environment it would download the new URI and add the checksum.
	// However, `DownloadAndChecksum` on `example.com` would fail.
	// In our code, if DownloadAndChecksum fails, it just skips that URI, leaving an empty manifest or
	// removing the old entry. We can just verify the old entry is removed.
}
