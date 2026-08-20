package g2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListConfiguredReposAndResolve(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Create a dummy overlay on disk with profiles/repo_name
	overlayDir := filepath.Join(tmpDir, "my-actual-overlay")
	if err := os.MkdirAll(filepath.Join(overlayDir, "profiles"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(overlayDir, "profiles", "repo_name"), []byte("actual-repo-identity\n"), 0644); err != nil {
		t.Fatalf("writing repo_name: %v", err)
	}

	// 2. Create repos.conf directory layout
	reposConfDir := filepath.Join(tmpDir, "repos.conf")
	if err := os.MkdirAll(reposConfDir, 0755); err != nil {
		t.Fatalf("mkdir repos.conf: %v", err)
	}

	f1Content := `[DEFAULT]
main-repo = gentoo

[alias-section]
location = ` + overlayDir + `

[regular-overlay]
location = /var/empty

# [comment-disabled]
# location = /var/empty
`
	if err := os.WriteFile(filepath.Join(reposConfDir, "custom.conf"), []byte(f1Content), 0644); err != nil {
		t.Fatalf("writing custom.conf: %v", err)
	}

	f2Content := `[flag-disabled]
location = /var/empty
disabled = true
`
	if err := os.WriteFile(filepath.Join(reposConfDir, "disabled.conf"), []byte(f2Content), 0644); err != nil {
		t.Fatalf("writing disabled.conf: %v", err)
	}

	// Test ListConfiguredRepos
	repos, err := ListConfiguredRepos(reposConfDir)
	if err != nil {
		t.Fatalf("ListConfiguredRepos failed: %v", err)
	}

	if len(repos) != 2 {
		t.Fatalf("expected 2 active repos, got %d", len(repos))
	}

	// Test ResolveRepo by alias section name
	r1, err := ResolveRepo("alias-section", reposConfDir)
	if err != nil {
		t.Fatalf("ResolveRepo(alias-section) failed: %v", err)
	}
	if r1.RepoName != "actual-repo-identity" {
		t.Errorf("RepoName = %q, want actual-repo-identity", r1.RepoName)
	}

	// Test ResolveRepo by actual repo identity
	r2, err := ResolveRepo("actual-repo-identity", reposConfDir)
	if err != nil {
		t.Fatalf("ResolveRepo(actual-repo-identity) failed: %v", err)
	}
	if r2.RepoName != "actual-repo-identity" {
		t.Errorf("RepoName = %q, want actual-repo-identity", r2.RepoName)
	}

	// Test ResolveRepo for regular overlay without profiles/repo_name
	r3, err := ResolveRepo("regular-overlay", reposConfDir)
	if err != nil {
		t.Fatalf("ResolveRepo(regular-overlay) failed: %v", err)
	}
	if r3.RepoName != "regular-overlay" {
		t.Errorf("RepoName = %q, want regular-overlay", r3.RepoName)
	}

	// Test disabled repo resolution fails
	if _, err := ResolveRepo("comment-disabled", reposConfDir); err == nil {
		t.Error("expected error resolving comment-disabled repo")
	}
	if _, err := ResolveRepo("flag-disabled", reposConfDir); err == nil {
		t.Error("expected error resolving flag-disabled repo")
	}

	// Test non-existent repo fails
	if _, err := ResolveRepo("non-existent", reposConfDir); err == nil {
		t.Error("expected error resolving non-existent repo")
	}

	// Test ambiguity
	ambigDir := t.TempDir()
	ambigConf := filepath.Join(ambigDir, "repos.conf")
	ambigContent := `[repo-a]
location = /var/db/repos/repo-a

[repo-b]
location = /var/db/repos/repo-b
`
	// Both point to different locations but if query matches multiple:
	if err := os.WriteFile(ambigConf, []byte(ambigContent), 0644); err != nil {
		t.Fatalf("writing ambig: %v", err)
	}
	// Let's create dummy repo_names that are both "common-identity"
	repoADir := filepath.Join(ambigDir, "repo-a")
	repoBDir := filepath.Join(ambigDir, "repo-b")
	_ = os.MkdirAll(filepath.Join(repoADir, "profiles"), 0755)
	_ = os.MkdirAll(filepath.Join(repoBDir, "profiles"), 0755)
	_ = os.WriteFile(filepath.Join(repoADir, "profiles", "repo_name"), []byte("common-identity"), 0644)
	_ = os.WriteFile(filepath.Join(repoBDir, "profiles", "repo_name"), []byte("common-identity"), 0644)

	ambigContent2 := `[repo-a]
location = ` + repoADir + `

[repo-b]
location = ` + repoBDir + `
`
	if err := os.WriteFile(ambigConf, []byte(ambigContent2), 0644); err != nil {
		t.Fatalf("writing ambig2: %v", err)
	}

	if _, err := ResolveRepo("common-identity", ambigConf); err == nil {
		t.Error("expected ambiguity error when two distinct repos share identity")
	}
}
