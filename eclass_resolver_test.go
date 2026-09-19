package g2

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

type denyingEclassFS struct{ fs.FS }

func (f denyingEclassFS) Open(name string) (fs.File, error) {
	if name == "eclass/shared.eclass" {
		return nil, os.ErrPermission
	}
	return f.FS.Open(name)
}

func writeResolverFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatalf("creating %s: %v", filepath.Dir(name), err)
	}
	if err := os.WriteFile(name, []byte(content), 0644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

func TestEclassResolver(t *testing.T) {
	// Setup mock filesystem for primary repo and masters
	dir := t.TempDir()

	// Primary Repo
	primaryDir := filepath.Join(dir, "primary")
	writeResolverFile(t, filepath.Join(primaryDir, "eclass", "primary-only.eclass"), "primary")
	writeResolverFile(t, filepath.Join(primaryDir, "eclass", "override.eclass"), "primary-override")

	// Gentoo Master Repo
	gentooDir := filepath.Join(dir, "gentoo")
	writeResolverFile(t, filepath.Join(gentooDir, "eclass", "gentoo-only.eclass"), "gentoo")
	writeResolverFile(t, filepath.Join(gentooDir, "eclass", "override.eclass"), "gentoo-override")

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

func TestEclassResolverDoesNotFallThroughReadFailure(t *testing.T) {
	primary := t.TempDir()
	master := t.TempDir()
	writeResolverFile(t, filepath.Join(primary, "eclass", "shared.eclass"), "primary")
	writeResolverFile(t, filepath.Join(master, "eclass", "shared.eclass"), "master")
	resolver := NewEclassResolver([]MasterRepo{
		{Name: "primary", Path: primary, FS: denyingEclassFS{os.DirFS(primary)}},
		{Name: "master", Path: master, FS: os.DirFS(master)},
	})
	if _, _, err := resolver.Resolve("shared"); err == nil {
		t.Fatal("read failure in high-precedence repository fell through to master")
	}
}

func TestBuildEclassResolverStrict(t *testing.T) {
	dir := t.TempDir()

	primaryDir := filepath.Join(dir, "primary")
	writeResolverFile(t, filepath.Join(primaryDir, "metadata", "layout.conf"), "masters = missing-master\n")

	cfs := NewOsCacheFS(primaryDir)

	policy := NewCachePolicy(CacheModeStrict)
	reposConfPath := filepath.Join(dir, "repos.conf")
	reposConfContent := `
[some-other-repo]
location = ` + primaryDir + `
`
	writeResolverFile(t, reposConfPath, reposConfContent)
	policy.ReposConfPath = reposConfPath

	_, err := BuildEclassResolver(cfs, ".", policy)
	if err == nil {
		t.Fatalf("Expected strict mode to fail when missing-master cannot be resolved")
	}
}

func TestBuildEclassResolverCI(t *testing.T) {
	dir := t.TempDir()

	primaryDir := filepath.Join(dir, "primary")
	writeResolverFile(t, filepath.Join(primaryDir, "metadata", "layout.conf"), "masters = missing-master\n")

	cfs := NewOsCacheFS(primaryDir)

	policy := NewCachePolicy(CacheModeCI)
	policy.ReposConfPath = filepath.Join(dir, "repos.conf")

	resolver, err := BuildEclassResolver(cfs, ".", policy)
	if err != nil {
		t.Fatalf("Expected CI mode to succeed even when master cannot be resolved, got %v", err)
	}
	if len(resolver.repos) != 1 {
		t.Fatalf("Expected 1 repo, got %d", len(resolver.repos))
	}
}

func TestBuildEclassResolverStrictNoMasters(t *testing.T) {
	dir := t.TempDir()

	primaryDir := filepath.Join(dir, "primary")
	if err := os.MkdirAll(filepath.Join(primaryDir, "metadata"), 0755); err != nil {
		t.Fatal(err)
	}
	// Missing masters intentionally!
	writeResolverFile(t, filepath.Join(primaryDir, "metadata", "layout.conf"), "")

	cfs := NewOsCacheFS(primaryDir)

	policy := NewCachePolicy(CacheModeStrict)
	reposConfPath := filepath.Join(dir, "repos.conf")
	writeResolverFile(t, reposConfPath, "")
	policy.ReposConfPath = reposConfPath

	resolver, err := BuildEclassResolver(cfs, ".", policy)
	if err != nil {
		t.Fatalf("Expected strict mode to succeed when no masters are declared, got error: %v", err)
	}
	if len(resolver.repos) != 1 {
		t.Fatalf("Expected 1 repo, got %d", len(resolver.repos))
	}
}

func TestBuildEclassResolverStrictExplicit(t *testing.T) {
	dir := t.TempDir()

	primaryDir := filepath.Join(dir, "primary")
	writeResolverFile(t, filepath.Join(primaryDir, "metadata", "layout.conf"), "masters = my-master\n")

	myMasterDir := filepath.Join(dir, "my-master")
	if err := os.MkdirAll(filepath.Join(myMasterDir, "eclass"), 0755); err != nil {
		t.Fatal(err)
	}

	cfs := NewOsCacheFS(primaryDir)

	policy := NewCachePolicy(CacheModeStrict)
	reposConfPath := filepath.Join(dir, "repos.conf")
	writeResolverFile(t, reposConfPath, "")
	policy.ReposConfPath = reposConfPath
	policy.ExplicitRepos["my-master"] = myMasterDir

	resolver, err := BuildEclassResolver(cfs, ".", policy)
	if err != nil {
		t.Fatalf("Expected strict mode to succeed when master is supplied via explicit repo, got error: %v", err)
	}
	if len(resolver.repos) != 2 {
		t.Fatalf("Expected 2 repos, got %d", len(resolver.repos))
	}
}

func TestBuildEclassResolverStrictReposConf(t *testing.T) {
	dir := t.TempDir()

	primaryDir := filepath.Join(dir, "primary")
	writeResolverFile(t, filepath.Join(primaryDir, "metadata", "layout.conf"), "masters = my-master\n")

	myMasterDir := filepath.Join(dir, "my-master")
	if err := os.MkdirAll(filepath.Join(myMasterDir, "eclass"), 0755); err != nil {
		t.Fatal(err)
	}

	cfs := NewOsCacheFS(primaryDir)

	policy := NewCachePolicy(CacheModeStrict)
	reposConfPath := filepath.Join(dir, "repos.conf")
	reposConfContent := `
[my-master]
location = ` + myMasterDir + `
`
	writeResolverFile(t, reposConfPath, reposConfContent)
	policy.ReposConfPath = reposConfPath

	resolver, err := BuildEclassResolver(cfs, ".", policy)
	if err != nil {
		t.Fatalf("Expected strict mode to succeed when master is supplied via repos.conf, got error: %v", err)
	}
	if len(resolver.repos) != 2 {
		t.Fatalf("Expected 2 repos, got %d", len(resolver.repos))
	}
}
