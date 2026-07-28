package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetNextRevision(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ebuild-next-revision-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	createEbuild := func(filename, content string) {
		path := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", filename, err)
		}
	}

	createEbuild("foo-1.0.ebuild", "EAPI=8\n# comment\nDEPEND=\"\"\n")
	createEbuild("foo-1.0-r1.ebuild", "EAPI=8\nDEPEND=\"a/b\"\n")
	createEbuild("foo-2.0.ebuild", "EAPI=8\n")

	// Create inspection file with identical significant content to 1.0-r1
	inspectMatch := filepath.Join(tmpDir, "inspect_match.ebuild")
	if err := os.WriteFile(inspectMatch, []byte("\n# new comment\nEAPI=8\n\nDEPEND=\"a/b\"\n"), 0644); err != nil {
		t.Fatalf("Failed to write inspectMatch: %v", err)
	}

	// Create inspection file with different content
	inspectDiffer := filepath.Join(tmpDir, "inspect_differ.ebuild")
	if err := os.WriteFile(inspectDiffer, []byte("EAPI=8\nDEPEND=\"a/c\"\n"), 0644); err != nil {
		t.Fatalf("Failed to write inspectDiffer: %v", err)
	}

	tests := []struct {
		name        string
		version     string
		inspectFile string
		expected    string
		expectCode  int
	}{
		{"No revision exists", "1.1", "", "1.1", 0},
		{"Existing base, no inspect", "2.0", "", "2.0-r1", 0},
		{"Existing revision, no inspect", "1.0", "", "1.0-r2", 0},
		{"Existing revision, inspect match", "1.0", inspectMatch, "1.0-r1", 1},
		{"Existing revision, inspect differ", "1.0", inspectDiffer, "1.0-r2", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, code, err := getNextRevision(tmpDir, tt.version, tt.inspectFile)
			if err != nil {
				t.Fatalf("getNextRevision returned unexpected error: %v", err)
			}
			if res != tt.expected {
				t.Errorf("expected output %v, got %v", tt.expected, res)
			}
			if code != tt.expectCode {
				t.Errorf("expected exit code %v, got %v", tt.expectCode, code)
			}
		})
	}
}
