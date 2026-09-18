package g2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEclassResolver(t *testing.T) {
	// Setup mock filesystem for primary repo and masters
	dir, err := os.MkdirTemp("", "g2-eclass-resolver-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(dir)

	// Primary Repo
	primaryDir := filepath.Join(dir, "primary")
	os.MkdirAll(filepath.Join(primaryDir, "eclass"), 0755)
	os.WriteFile(filepath.Join(primaryDir, "eclass", "primary-only.eclass"), []byte("primary"), 0644)
	os.WriteFile(filepath.Join(primaryDir, "eclass", "override.eclass"), []byte("primary-override"), 0644)

	// Gentoo Master Repo
	gentooDir := filepath.Join(dir, "gentoo")
	os.MkdirAll(filepath.Join(gentooDir, "eclass"), 0755)
	os.WriteFile(filepath.Join(gentooDir, "eclass", "gentoo-only.eclass"), []byte("gentoo"), 0644)
	os.WriteFile(filepath.Join(gentooDir, "eclass", "override.eclass"), []byte("gentoo-override"), 0644)

	cfs := NewOsCacheFS(primaryDir)

	repos := []MasterRepo{
		{Name: "primary", Path: primaryDir, FS: cfs},
		{Name: "gentoo", Path: gentooDir, FS: os.DirFS(gentooDir)},
	}

	resolver := NewEclassResolver(repos)

	tests := []struct {
		name         string
		expected     string
		expectedRepo string
		expectErr    bool
	}{
		{"primary-only", "primary", "primary", false},
		{"gentoo-only", "gentoo", "gentoo", false},
		{"override", "primary-override", "primary", false},
		{"non-existent", "", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			content, repoName, err := resolver.Resolve(tc.name)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if string(content) != tc.expected {
					t.Errorf("expected content %q, got %q", tc.expected, string(content))
				}
				if repoName != tc.expectedRepo {
					t.Errorf("expected repo %q, got %q", tc.expectedRepo, repoName)
				}
			}
		})
	}
}

func TestBuildEclassResolverStrict(t *testing.T) {
	dir, err := os.MkdirTemp("", "g2-eclass-resolver-strict-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(dir)

	primaryDir := filepath.Join(dir, "primary")
	os.MkdirAll(filepath.Join(primaryDir, "metadata"), 0755)
	os.WriteFile(filepath.Join(primaryDir, "metadata", "layout.conf"), []byte("masters = missing-master\n"), 0644)

	cfs := NewOsCacheFS(primaryDir)

	policy := NewCachePolicy(CacheModeStrict)
	reposConfPath := filepath.Join(dir, "repos.conf")
	reposConfContent := `
[some-other-repo]
location = ` + primaryDir + `
`
	os.WriteFile(reposConfPath, []byte(reposConfContent), 0644)
	policy.ReposConfPath = reposConfPath

	_, err = BuildEclassResolver(cfs, primaryDir, policy)
	if err == nil {
		t.Fatalf("Expected strict mode to fail when missing-master cannot be resolved")
	}
}

func TestBuildEclassResolverCI(t *testing.T) {
	dir, err := os.MkdirTemp("", "g2-eclass-resolver-ci-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(dir)

	primaryDir := filepath.Join(dir, "primary")
	os.MkdirAll(filepath.Join(primaryDir, "metadata"), 0755)
	os.WriteFile(filepath.Join(primaryDir, "metadata", "layout.conf"), []byte("masters = missing-master\n"), 0644)

	cfs := NewOsCacheFS(primaryDir)

	policy := NewCachePolicy(CacheModeCI)
	policy.ReposConfPath = filepath.Join(dir, "repos.conf")

	resolver, err := BuildEclassResolver(cfs, primaryDir, policy)
	if err != nil {
		t.Fatalf("Expected CI mode to succeed even when master cannot be resolved, got %v", err)
	}
	if len(resolver.repos) != 1 {
		t.Fatalf("Expected 1 repo, got %d", len(resolver.repos))
	}
}

func TestBuildEclassResolverStrictNoMasters(t *testing.T) {
	dir, err := os.MkdirTemp("", "g2-eclass-resolver-strict-no-masters-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(dir)

	primaryDir := filepath.Join(dir, "primary")
	os.MkdirAll(filepath.Join(primaryDir, "metadata"), 0755)
	// Missing masters intentionally!
	os.WriteFile(filepath.Join(primaryDir, "metadata", "layout.conf"), []byte(""), 0644)

	cfs := NewOsCacheFS(primaryDir)

	policy := NewCachePolicy(CacheModeStrict)
	reposConfPath := filepath.Join(dir, "repos.conf")
	os.WriteFile(reposConfPath, []byte(""), 0644)
	policy.ReposConfPath = reposConfPath

	resolver, err := BuildEclassResolver(cfs, primaryDir, policy)
	if err != nil {
		t.Fatalf("Expected strict mode to succeed when no masters are declared, got error: %v", err)
	}
	if len(resolver.repos) != 1 {
		t.Fatalf("Expected 1 repo, got %d", len(resolver.repos))
	}
}

func TestBuildEclassResolverStrictExplicit(t *testing.T) {
	dir, err := os.MkdirTemp("", "g2-eclass-resolver-strict-explicit-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(dir)

	primaryDir := filepath.Join(dir, "primary")
	os.MkdirAll(filepath.Join(primaryDir, "metadata"), 0755)
	os.WriteFile(filepath.Join(primaryDir, "metadata", "layout.conf"), []byte("masters = my-master\n"), 0644)

	myMasterDir := filepath.Join(dir, "my-master")
	os.MkdirAll(filepath.Join(myMasterDir, "eclass"), 0755)

	cfs := NewOsCacheFS(primaryDir)

	policy := NewCachePolicy(CacheModeStrict)
	reposConfPath := filepath.Join(dir, "repos.conf")
	os.WriteFile(reposConfPath, []byte(""), 0644)
	policy.ReposConfPath = reposConfPath
	policy.ExplicitRepos["my-master"] = myMasterDir

	resolver, err := BuildEclassResolver(cfs, primaryDir, policy)
	if err != nil {
		t.Fatalf("Expected strict mode to succeed when master is supplied via explicit repo, got error: %v", err)
	}
	if len(resolver.repos) != 2 {
		t.Fatalf("Expected 2 repos, got %d", len(resolver.repos))
	}
}

func TestBuildEclassResolverStrictReposConf(t *testing.T) {
	dir, err := os.MkdirTemp("", "g2-eclass-resolver-strict-reposconf-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(dir)

	primaryDir := filepath.Join(dir, "primary")
	os.MkdirAll(filepath.Join(primaryDir, "metadata"), 0755)
	os.WriteFile(filepath.Join(primaryDir, "metadata", "layout.conf"), []byte("masters = my-master\n"), 0644)

	myMasterDir := filepath.Join(dir, "my-master")
	os.MkdirAll(filepath.Join(myMasterDir, "eclass"), 0755)

	cfs := NewOsCacheFS(primaryDir)

	policy := NewCachePolicy(CacheModeStrict)
	reposConfPath := filepath.Join(dir, "repos.conf")
	reposConfContent := `
[my-master]
location = ` + myMasterDir + `
`
	os.WriteFile(reposConfPath, []byte(reposConfContent), 0644)
	policy.ReposConfPath = reposConfPath

	resolver, err := BuildEclassResolver(cfs, primaryDir, policy)
	if err != nil {
		t.Fatalf("Expected strict mode to succeed when master is supplied via repos.conf, got error: %v", err)
	}
	if len(resolver.repos) != 2 {
		t.Fatalf("Expected 2 repos, got %d", len(resolver.repos))
	}
}
