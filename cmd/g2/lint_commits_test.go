package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGetGitModifiedPackages(t *testing.T) {
	// Create a temporary git repository
	tmpDir, err := os.MkdirTemp("", "g2-git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	runCmd := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpDir
		err := cmd.Run()
		if err != nil {
			t.Fatalf("Command %v failed: %v", args, err)
		}
	}

	runCmd("git", "init")

	// Create initial commit
	err = os.MkdirAll(filepath.Join(tmpDir, "app-misc", "foo"), 0755)
	if err != nil { t.Fatal(err) }
	err = os.WriteFile(filepath.Join(tmpDir, "app-misc", "foo", "foo-1.ebuild"), []byte(""), 0644)
	if err != nil { t.Fatal(err) }

	runCmd("git", "add", ".")
	runCmd("git", "commit", "-m", "Initial commit")

	// Now modify a file
	err = os.WriteFile(filepath.Join(tmpDir, "app-misc", "foo", "foo-1.ebuild"), []byte("modified"), 0644)
	if err != nil { t.Fatal(err) }

	// Add a new file (untracked)
	err = os.MkdirAll(filepath.Join(tmpDir, "dev-util", "bar"), 0755)
	if err != nil { t.Fatal(err) }
	err = os.WriteFile(filepath.Join(tmpDir, "dev-util", "bar", "bar-2.ebuild"), []byte(""), 0644)
	if err != nil { t.Fatal(err) }

	pkgs, err := getGitModifiedPackages(tmpDir)
	if err != nil {
		t.Fatalf("getGitModifiedPackages failed: %v", err)
	}

	foundFoo := false
	foundBar := false
	for _, p := range pkgs {
		if p == "app-misc/foo" {
			foundFoo = true
		}
		if p == "dev-util/bar" {
			foundBar = true
		}
	}

	if !foundFoo {
		t.Errorf("Expected to find app-misc/foo in modified packages, got %v", pkgs)
	}
	if !foundBar {
		t.Errorf("Expected to find dev-util/bar in modified packages, got %v", pkgs)
	}
}
