package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEbuildTag(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "ebuild-tag-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create some fake ebuilds
	versions := []string{"1.0.0", "1.1.0", "1.1.1", "2.0.0-r1", "2.0.0"}
	pn := "testpkg"

	for _, v := range versions {
		ebuildPath := filepath.Join(tmpDir, pn+"-"+v+".ebuild")
		if err := os.WriteFile(ebuildPath, []byte("EAPI=8"), 0644); err != nil {
			t.Fatalf("Failed to create fake ebuild %s: %v", ebuildPath, err)
		}
	}

	cfg := &CmdEbuildArgConfig{}

	var buf bytes.Buffer

	// Test basic execution without compare (should return highest version)
	err = cfg.cmdEbuildTag([]string{"-dir", tmpDir}, &buf)
	if err != nil {
		t.Fatalf("cmdEbuildTag failed: %v", err)
	}

	output := strings.TrimSpace(buf.String())

	if output != "2.0.0-r1" {
		t.Errorf("Expected highest version to be 2.0.0-r1, got %s", output)
	}

	// Test compare downgrade without flag
	buf.Reset()
	err = cfg.cmdEbuildTag([]string{"-dir", tmpDir, "-compare", "1.5.0"}, &buf)
	if err != nil {
		t.Fatalf("cmdEbuildTag failed: %v", err)
	}
	output = strings.TrimSpace(buf.String())
	if output != "" {
		t.Errorf("Expected empty output for downgrade without -downgrades flag, got %s", output)
	}

	// Test compare downgrade with flag
	buf.Reset()
	err = cfg.cmdEbuildTag([]string{"-dir", tmpDir, "-compare", "1.5.0", "-downgrades"}, &buf)
	if err != nil {
		t.Fatalf("cmdEbuildTag failed: %v", err)
	}
	output = strings.TrimSpace(buf.String())
	if output != "-" {
		t.Errorf("Expected '-' output for downgrade with -downgrades flag, got %s", output)
	}

	// Test compare equal
	buf.Reset()
	err = cfg.cmdEbuildTag([]string{"-dir", tmpDir, "-compare", "2.0.0-r1"}, &buf)
	if err != nil {
		t.Fatalf("cmdEbuildTag failed: %v", err)
	}
	output = strings.TrimSpace(buf.String())
	if output != "=" {
		t.Errorf("Expected '=' output for equal version, got %s", output)
	}

	// Test compare upgrade
	buf.Reset()
	err = cfg.cmdEbuildTag([]string{"-dir", tmpDir, "-compare", "2.1.0"}, &buf)
	if err != nil {
		t.Fatalf("cmdEbuildTag failed: %v", err)
	}
	output = strings.TrimSpace(buf.String())
	if output != "+" {
		t.Errorf("Expected '+' output for upgrade version, got %s", output)
	}

	// Test patch bump
	buf.Reset()
	err = cfg.cmdEbuildTag([]string{"-dir", tmpDir, "-patch"}, &buf)
	if err != nil {
		t.Fatalf("cmdEbuildTag failed: %v", err)
	}
	output = strings.TrimSpace(buf.String())
	if output != "2.0.1" { // 2.0.0-r1 -> micro increment drops suffix and goes to next number
		t.Errorf("Expected '2.0.1' output for patch bump, got %s", output)
	}

	// Test revision bump
	buf.Reset()
	err = cfg.cmdEbuildTag([]string{"-dir", tmpDir, "-revision"}, &buf)
	if err != nil {
		t.Fatalf("cmdEbuildTag failed: %v", err)
	}
	output = strings.TrimSpace(buf.String())
	if output != "2.0.0-r2" {
		t.Errorf("Expected '2.0.0-r2' output for revision bump, got %s", output)
	}
}
