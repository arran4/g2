package main

import (
	"bytes"
	"io/fs"
	"os"
	"testing"
)

func TestCmdEbuildInstall(t *testing.T) {
	mockFS := NewMockFS()

	ebuildContent := []byte(`EAPI=8
CATEGORY="app-test"
DESCRIPTION="Test"
`)
	// Setup repo dir
	mockFS.MkdirAll("repo", 0755)

	cfg := &CmdEbuildArgConfig{
		MainArgConfig: &MainArgConfig{},
	}

	// Install from stdin
	stdin := bytes.NewReader(ebuildContent)

	if err := cfg.InstallEbuild(mockFS, []string{"-", "test-pkg-1.0.ebuild"}, "repo", stdin); err != nil {
		t.Fatalf("InstallEbuild failed: %v", err)
	}

	targetFile := "repo/app-test/test-pkg/test-pkg-1.0.ebuild"
	if _, err := fs.Stat(mockFS, targetFile); os.IsNotExist(err) {
		t.Errorf("Ebuild was not installed to expected path: %s", targetFile)
	}
}
