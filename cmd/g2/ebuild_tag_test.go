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

	// Test basic execution without compare (should return highest version)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = cfg.cmdEbuildTag([]string{"-dir", tmpDir})
	if err != nil {
		t.Fatalf("cmdEbuildTag failed: %v", err)
	}

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := strings.TrimSpace(buf.String())

	if output != "2.0.0-r1" {
		t.Errorf("Expected highest version to be 2.0.0-r1, got %s", output)
	}

	// Test compare downgrade without flag
	r2, w2, _ := os.Pipe()
	os.Stdout = w2
	err = cfg.cmdEbuildTag([]string{"-dir", tmpDir, "-compare", "1.5.0"})
	if err != nil {
		t.Fatalf("cmdEbuildTag failed: %v", err)
	}
	_ = w2.Close()
	os.Stdout = oldStdout
	buf.Reset()
	_, _ = buf.ReadFrom(r2)
	output = strings.TrimSpace(buf.String())
	if output != "" {
		t.Errorf("Expected empty output for downgrade without -downgrades flag, got %s", output)
	}

	// Test compare downgrade with flag
	r3, w3, _ := os.Pipe()
	os.Stdout = w3
	err = cfg.cmdEbuildTag([]string{"-dir", tmpDir, "-compare", "1.5.0", "-downgrades"})
	if err != nil {
		t.Fatalf("cmdEbuildTag failed: %v", err)
	}
	_ = w3.Close()
	os.Stdout = oldStdout
	buf.Reset()
	_, _ = buf.ReadFrom(r3)
	output = strings.TrimSpace(buf.String())
	if output != "-" {
		t.Errorf("Expected '-' output for downgrade with -downgrades flag, got %s", output)
	}

	// Test compare equal
	r4, w4, _ := os.Pipe()
	os.Stdout = w4
	err = cfg.cmdEbuildTag([]string{"-dir", tmpDir, "-compare", "2.0.0-r1"})
	if err != nil {
		t.Fatalf("cmdEbuildTag failed: %v", err)
	}
	_ = w4.Close()
	os.Stdout = oldStdout
	buf.Reset()
	_, _ = buf.ReadFrom(r4)
	output = strings.TrimSpace(buf.String())
	if output != "=" {
		t.Errorf("Expected '=' output for equal version, got %s", output)
	}

	// Test compare upgrade
	r5, w5, _ := os.Pipe()
	os.Stdout = w5
	err = cfg.cmdEbuildTag([]string{"-dir", tmpDir, "-compare", "2.1.0"})
	if err != nil {
		t.Fatalf("cmdEbuildTag failed: %v", err)
	}
	_ = w5.Close()
	os.Stdout = oldStdout
	buf.Reset()
	_, _ = buf.ReadFrom(r5)
	output = strings.TrimSpace(buf.String())
	if output != "+" {
		t.Errorf("Expected '+' output for upgrade version, got %s", output)
	}

	// Test patch bump
	r6, w6, _ := os.Pipe()
	os.Stdout = w6
	err = cfg.cmdEbuildTag([]string{"-dir", tmpDir, "-patch"})
	if err != nil {
		t.Fatalf("cmdEbuildTag failed: %v", err)
	}
	_ = w6.Close()
	os.Stdout = oldStdout
	buf.Reset()
	_, _ = buf.ReadFrom(r6)
	output = strings.TrimSpace(buf.String())
	if output != "2.0.1" { // 2.0.0-r1 -> micro increment drops suffix and goes to next number
		t.Errorf("Expected '2.0.1' output for patch bump, got %s", output)
	}

	// Test revision bump
	r7, w7, _ := os.Pipe()
	os.Stdout = w7
	err = cfg.cmdEbuildTag([]string{"-dir", tmpDir, "-revision"})
	if err != nil {
		t.Fatalf("cmdEbuildTag failed: %v", err)
	}
	_ = w7.Close()
	os.Stdout = oldStdout
	buf.Reset()
	_, _ = buf.ReadFrom(r7)
	output = strings.TrimSpace(buf.String())
	if output != "2.0.0-r2" {
		t.Errorf("Expected '2.0.0-r2' output for revision bump, got %s", output)
	}
}
