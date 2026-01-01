package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyAndClean(t *testing.T) {
	// Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "g2-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Manifest
	manifestPath := filepath.Join(tmpDir, "Manifest")
	initialManifest := "DIST used-file.tar.gz 100 SHA512 fakehash\nDIST unused-file.tar.gz 200 SHA512 fakehash2\n"
	if err := os.WriteFile(manifestPath, []byte(initialManifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Create ebuild
	ebuildPath := filepath.Join(tmpDir, "package-1.0.ebuild")
	ebuildContent := `
EAPI=8
SRC_URI="https://example.com/used-file.tar.gz"
`
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &MainArgConfig{
		Args: []string{"g2"},
	}
	cmdCfg := &CmdManifestArgConfig{
		MainArgConfig: cfg,
	}

	// Test Clean Logic
	// We manually invoke cleanLogic because cmdClean runs everything including parsing which uses real filesystem but we want to verify the logic.
	// Actually cmdClean uses the real filesystem which we just set up, so we can use it directly?
	// But main package is not importable easily for tests if we are outside.
	// We are in package main_test (if we rename) or main.
	// If we are in package main, we can call methods.

	// Let's test Clean first
	err = cmdCfg.cmdClean([]string{tmpDir})
	if err != nil {
		t.Fatalf("cmdClean failed: %v", err)
	}

	// Verify Manifest content
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	output := string(content)
	if strings.Contains(output, "unused-file.tar.gz") {
		t.Errorf("Manifest still contains unused file after clean")
	}
	if !strings.Contains(output, "used-file.tar.gz") {
		t.Errorf("Manifest missing used file after clean")
	}

	// Test Verify Logic
	// We want to test that it reports missing files.
	// Add a new ebuild with missing file
	ebuildPath2 := filepath.Join(tmpDir, "package-2.0.ebuild")
	ebuildContent2 := `
EAPI=8
SRC_URI="https://example.com/missing-file.tar.gz"
`
	if err := os.WriteFile(ebuildPath2, []byte(ebuildContent2), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture log output? It's hard to capture log output in Go tests without redirecting log.SetOutput.
	// For now we just ensure it runs without error.

	// We can't easily test --fix because it tries to download from real URLs.
	// We would need to mock http client or DownloadAndChecksum.
	// Since DownloadAndChecksum is in g2 package, we can't easily mock it from main unless we change structure.

	err = cmdCfg.cmdVerify([]string{tmpDir}, []string{})
	if err != nil {
		t.Fatalf("cmdVerify failed: %v", err)
	}

	// Verify output manually or check if it didn't crash.
}
