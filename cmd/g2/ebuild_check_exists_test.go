package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckEbuildExists(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "ebuild-check-exists-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Helper to create empty ebuild files
	createEbuild := func(filename string) {
		path := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", filename, err)
		}
	}

	createEbuild("foo-1.2.ebuild")
	createEbuild("foo-1.3-r1.ebuild")
	createEbuild("foo-2.0.0-r10.ebuild")
	createEbuild("foo-bar-1.2.ebuild")

	tests := []struct {
		version  string
		expected bool
	}{
		{"1.2", true},      // Exact match base version
		{"1.3", true},      // Matches with -r1 revision
		{"2.0.0", true},    // Matches with -r10 revision
		{"1.4", false},     // Does not exist
		{"2.0", false},     // Substring of 2.0.0, shouldn't match
		{"bar-1.2", false}, // It's a package name part, not version
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			exists, err := checkEbuildExists(tmpDir, tt.version)
			if err != nil {
				t.Fatalf("checkEbuildExists returned unexpected error: %v", err)
			}
			if exists != tt.expected {
				t.Errorf("expected %v, got %v for version %s", tt.expected, exists, tt.version)
			}
		})
	}
}
