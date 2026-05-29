package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorldReadWrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "g2-world-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	worldPath := filepath.Join(tmpDir, "world")

	// Test write
	lines := []string{"app-editors/vim", "# dev-lang/go", "sys-kernel/gentoo-sources"}
	if err := writeWorldFile(worldPath, lines); err != nil {
		t.Fatalf("failed to write world file: %v", err)
	}

	// Test read
	readLines, err := readWorldFile(worldPath)
	if err != nil {
		t.Fatalf("failed to read world file: %v", err)
	}

	if len(readLines) != len(lines) {
		t.Fatalf("expected %d lines, got %d", len(lines), len(readLines))
	}

	for i, line := range lines {
		if readLines[i] != line {
			t.Errorf("line %d: expected %q, got %q", i, line, readLines[i])
		}
	}
}
