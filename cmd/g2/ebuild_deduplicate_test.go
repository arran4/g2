package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCmdEbuildDeduplicate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "g2-dedup-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	pkgDir := filepath.Join(tmpDir, "app-test", "dummy")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create pkg dir: %v", err)
	}

	ebuilds := map[string]string{
		"dummy-1.0.ebuild":    "EAPI=8\nSLOT=\"0\"\n# Generated via: some-tool\nDESCRIPTION=\"Test\"\n",
		"dummy-1.1.ebuild":    "EAPI=8\nSLOT=\"0\"\n# Generated via: some-tool\nDESCRIPTION=\"Test\"\n",
		"dummy-1.2.ebuild":    "EAPI=8\nSLOT=\"0\"\n# Generated via: some-tool\nDESCRIPTION=\"Test\"\n",
		"dummy-2.0.ebuild":    "EAPI=8\nSLOT=\"1\"\n# Generated via: some-tool\nDESCRIPTION=\"Test 2\"\n",
		"dummy-9999.ebuild":   "EAPI=8\nSLOT=\"0\"\n# Generated via: some-tool\nDESCRIPTION=\"Live\"\n",
		"dummy-1.2-r1.ebuild": "EAPI=8\nSLOT=\"0\"\n# Generated via: some-tool\nDESCRIPTION=\"Test diff\"\n",
	}

	for name, content := range ebuilds {
		if err := os.WriteFile(filepath.Join(pkgDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	// Create manifest
	if err := os.WriteFile(filepath.Join(pkgDir, "Manifest"), []byte("DIST dummy-1.0.tar.gz 123 BLAKE2B xyz\n"), 0644); err != nil {
		t.Fatalf("failed to write Manifest: %v", err)
	}

	cfg := &CmdEbuildArgConfig{
		MainArgConfig: &MainArgConfig{},
	}

	if err := cfg.cmdEbuildDeduplicate([]string{pkgDir}); err != nil {
		t.Fatalf("deduplicate failed: %v", err)
	}

	// 1.0, 1.1, 1.2 have SAME DIGEST. So 1.0 and 1.1 should be removed, 1.2 kept.
	// 1.2-r1 has DIFFERENT DIGEST. But both 1.2 and 1.2-r1 are in same slot/grade. So older (1.2) should be removed.
	// 9999 is in grade 9999, so kept.
	// 2.0 is in slot 1, so kept.

	expectedFiles := []string{
		"dummy-1.2-r1.ebuild",
		"dummy-2.0.ebuild",
		"dummy-9999.ebuild",
		"Manifest",
	}

	entries, _ := os.ReadDir(pkgDir)
	var foundFiles []string
	for _, e := range entries {
		foundFiles = append(foundFiles, e.Name())
	}

	expectedMap := make(map[string]bool)
	for _, f := range expectedFiles {
		expectedMap[f] = true
	}

	for _, f := range foundFiles {
		if !expectedMap[f] {
			t.Errorf("found unexpected file: %s", f)
		}
	}

	for _, f := range expectedFiles {
		found := false
		for _, foundF := range foundFiles {
			if f == foundF {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected file not found: %s", f)
		}
	}
}
