package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGetGitModifiedPackagesChanged(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "g2-git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	runCmd := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpDir
		err := cmd.Run()
		if err != nil {
			t.Fatalf("Command %v failed: %v", args, err)
		}
	}

	// 1. Not a git work tree
	_, err = getGitModifiedPackagesChanged(tmpDir, "")
	if err == nil {
		t.Errorf("Expected error for non-git directory")
	}

	runCmd("git", "init")

	// 2. Initial state
	err = os.MkdirAll(filepath.Join(tmpDir, "app-misc", "foo"), 0755)
	if err != nil { t.Fatal(err) }
	err = os.WriteFile(filepath.Join(tmpDir, "app-misc", "foo", "foo-1.ebuild"), []byte(""), 0644)
	if err != nil { t.Fatal(err) }

	runCmd("git", "add", ".")
	runCmd("git", "commit", "-m", "Initial commit")

	runCmd("git", "branch", "-M", "main")

	// 3. Invalid base
	_, err = getGitModifiedPackagesChanged(tmpDir, "nonexistent-branch")
	if err == nil {
		t.Errorf("Expected error for invalid explicit base")
	}

	// 4. Create an unstaged modification
	err = os.WriteFile(filepath.Join(tmpDir, "app-misc", "foo", "foo-1.ebuild"), []byte("modified"), 0644)
	if err != nil { t.Fatal(err) }

	// 5. Create a staged modification (rename/add)
	err = os.MkdirAll(filepath.Join(tmpDir, "dev-util", "bar"), 0755)
	if err != nil { t.Fatal(err) }
	err = os.WriteFile(filepath.Join(tmpDir, "dev-util", "bar", "bar-2.ebuild"), []byte("staged"), 0644)
	if err != nil { t.Fatal(err) }
	runCmd("git", "add", "dev-util/bar/bar-2.ebuild")

	// 6. Create an untracked file
	err = os.MkdirAll(filepath.Join(tmpDir, "sys-apps", "baz"), 0755)
	if err != nil { t.Fatal(err) }
	err = os.WriteFile(filepath.Join(tmpDir, "sys-apps", "baz", "baz-3.ebuild"), []byte("untracked"), 0644)
	if err != nil { t.Fatal(err) }

	// 7. Whitespace filename
	err = os.MkdirAll(filepath.Join(tmpDir, "sys-apps", "white space"), 0755)
	if err != nil { t.Fatal(err) }
	err = os.WriteFile(filepath.Join(tmpDir, "sys-apps", "white space", "white space-3.ebuild"), []byte("untracked whitespace"), 0644)
	if err != nil { t.Fatal(err) }

	// 8. Repository metadata change (should be ignored)
	err = os.MkdirAll(filepath.Join(tmpDir, "metadata"), 0755)
	if err != nil { t.Fatal(err) }
	err = os.WriteFile(filepath.Join(tmpDir, "metadata", "layout.conf"), []byte(""), 0644)
	if err != nil { t.Fatal(err) }

	// Run without explicit base (no upstream configured)
	pkgs, err := getGitModifiedPackagesChanged(tmpDir, "")
	if err != nil {
		t.Fatalf("getGitModifiedPackagesChanged failed: %v", err)
	}

	foundMap := make(map[string]bool)
	for _, p := range pkgs {
		foundMap[p] = true
	}

	if !foundMap["app-misc/foo"] { t.Errorf("Missing unstaged app-misc/foo") }
	if !foundMap["dev-util/bar"] { t.Errorf("Missing staged dev-util/bar") }
	if !foundMap["sys-apps/baz"] { t.Errorf("Missing untracked sys-apps/baz") }
	if !foundMap["sys-apps/white space"] { t.Errorf("Missing whitespace sys-apps/white space") }
	if foundMap["metadata/layout.conf"] || foundMap["metadata"] {
		t.Errorf("Should not include metadata/layout.conf as a package")
	}

	// 9. Committed branch change with upstream
	runCmd("git", "checkout", "-b", "feature")
	err = os.WriteFile(filepath.Join(tmpDir, "sys-apps", "baz", "baz-3.ebuild"), []byte("committed"), 0644)
	if err != nil { t.Fatal(err) }
	runCmd("git", "add", "sys-apps/baz/baz-3.ebuild")
	runCmd("git", "commit", "-m", "Add baz")
	runCmd("git", "branch", "--set-upstream-to=main", "feature")

	// 10. Deleted package file
	// Actually we wrote to app-misc/foo/foo-1.ebuild, but we didn't add it after modifying it in step 4
	// So to use git rm on it properly, it needs to be known. Let's just rm it forcefully using standard OS
	err = os.Remove(filepath.Join(tmpDir, "app-misc", "foo", "foo-1.ebuild"))
	if err != nil { t.Fatal(err) }

	pkgs, err = getGitModifiedPackagesChanged(tmpDir, "")
	if err != nil {
		t.Fatalf("getGitModifiedPackagesChanged failed: %v", err)
	}

	foundMap = make(map[string]bool)
	for _, p := range pkgs {
		foundMap[p] = true
	}

	if !foundMap["sys-apps/baz"] { t.Errorf("Missing committed sys-apps/baz") }
	if !foundMap["app-misc/foo"] { t.Errorf("Missing deleted app-misc/foo") }
	if !foundMap["dev-util/bar"] { t.Errorf("Missing staged dev-util/bar") }

	// 11. Explicit base
	pkgs, err = getGitModifiedPackagesChanged(tmpDir, "main")
	if err != nil {
		t.Fatalf("getGitModifiedPackagesChanged explicit base failed: %v", err)
	}
	foundMap = make(map[string]bool)
	for _, p := range pkgs {
		foundMap[p] = true
	}
	if !foundMap["sys-apps/baz"] { t.Errorf("Missing committed sys-apps/baz against explicit base main") }
}
