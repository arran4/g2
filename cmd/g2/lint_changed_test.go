package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/arran4/g2"
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

	siteData := &g2.SiteData{
		Categories: []g2.CategoryData{
			{Name: "app-misc"},
			{Name: "dev-util"},
			{Name: "sys-apps"},
		},
	}

	// 1. Not a git work tree
	_, err = getGitModifiedPackagesChanged(tmpDir, "", siteData)
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
	_, err = getGitModifiedPackagesChanged(tmpDir, "nonexistent-branch", siteData)
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

	// Also some random infra file
	err = os.MkdirAll(filepath.Join(tmpDir, ".github", "workflows"), 0755)
	if err != nil { t.Fatal(err) }
	err = os.WriteFile(filepath.Join(tmpDir, ".github", "workflows", "ci.yml"), []byte(""), 0644)
	if err != nil { t.Fatal(err) }

	// And a script path that isn't a category
	err = os.MkdirAll(filepath.Join(tmpDir, "scripts", "foo"), 0755)
	if err != nil { t.Fatal(err) }
	err = os.WriteFile(filepath.Join(tmpDir, "scripts", "foo", "bar"), []byte(""), 0644)
	if err != nil { t.Fatal(err) }

	// Run without explicit base (no upstream configured)
	pkgs, err := getGitModifiedPackagesChanged(tmpDir, "", siteData)
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
	if foundMap["metadata/layout.conf"] || foundMap["metadata"] || foundMap[".github/workflows"] || foundMap["scripts/foo"] {
		t.Errorf("Should not include non-package infrastructure files as packages")
	}

	// 9. Committed branch change with upstream
	runCmd("git", "checkout", "-b", "feature")
	err = os.WriteFile(filepath.Join(tmpDir, "sys-apps", "baz", "baz-3.ebuild"), []byte("committed"), 0644)
	if err != nil { t.Fatal(err) }
	runCmd("git", "add", "sys-apps/baz/baz-3.ebuild")
	runCmd("git", "commit", "-m", "Add baz")
	runCmd("git", "branch", "--set-upstream-to=main", "feature")

	// 10. Genuine git rename
	runCmd("git", "mv", "app-misc/foo", "app-misc/foo2")

	// 11. Explicit base
	pkgs, err = getGitModifiedPackagesChanged(tmpDir, "main", siteData)
	if err != nil {
		t.Fatalf("getGitModifiedPackagesChanged explicit base failed: %v", err)
	}
	foundMap = make(map[string]bool)
	for _, p := range pkgs {
		foundMap[p] = true
	}
	if !foundMap["sys-apps/baz"] { t.Errorf("Missing committed sys-apps/baz against explicit base main") }
	if !foundMap["app-misc/foo"] { t.Errorf("Missing rename source app-misc/foo") }
	if !foundMap["app-misc/foo2"] { t.Errorf("Missing rename destination app-misc/foo2") }

	// Test deterministic order
	isSorted := true
	for i := 1; i < len(pkgs); i++ {
		if pkgs[i-1] > pkgs[i] {
			isSorted = false
			break
		}
	}
	if !isSorted {
		t.Errorf("Package list is not sorted: %v", pkgs)
	}
}
