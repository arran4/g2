package ebuild

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arran4/g2"
)

func TestRepoNameMissingLintRule(t *testing.T) {
	category := "app-test"
	name := "testpkg"

	pkgData := &g2.PackageData{
		Category: category,
		Name:     name,
	}

	tests := []struct {
		name         string
		setupRepo    func(repoDir string)
		expectErrors int
	}{
		{
			name: "Missing repo-name",
			setupRepo: func(repoDir string) {
				// Create metadata and profiles dirs, but leave them empty
				os.MkdirAll(filepath.Join(repoDir, "metadata"), 0755)
				os.MkdirAll(filepath.Join(repoDir, "profiles"), 0755)
			},
			expectErrors: 1,
		},
		{
			name: "Present in layout.conf",
			setupRepo: func(repoDir string) {
				metadataDir := filepath.Join(repoDir, "metadata")
				os.MkdirAll(metadataDir, 0755)
				layoutConf := "repo-name = test-overlay\nmasters = gentoo\n"
				os.WriteFile(filepath.Join(metadataDir, "layout.conf"), []byte(layoutConf), 0644)
			},
			expectErrors: 0,
		},
		{
			name: "Present in profiles/repo_name",
			setupRepo: func(repoDir string) {
				profilesDir := filepath.Join(repoDir, "profiles")
				os.MkdirAll(profilesDir, 0755)
				os.WriteFile(filepath.Join(profilesDir, "repo_name"), []byte("test-overlay\n"), 0644)
			},
			expectErrors: 0,
		},
		{
			name: "Empty layout.conf repo-name",
			setupRepo: func(repoDir string) {
				metadataDir := filepath.Join(repoDir, "metadata")
				os.MkdirAll(metadataDir, 0755)
				layoutConf := "repo-name =\nmasters = gentoo\n"
				os.WriteFile(filepath.Join(metadataDir, "layout.conf"), []byte(layoutConf), 0644)
			},
			expectErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "repo-name-test")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			tt.setupRepo(tempDir)

			rule := &RepoNameMissingLintRule{}
			results := rule.Lint(tempDir, pkgData)

			if len(results) != tt.expectErrors {
				t.Errorf("expected %d errors, got %d", tt.expectErrors, len(results))
				for _, r := range results {
					t.Log(r.Message)
				}
			}

			for _, result := range results {
				if result.RuleMetadata.ID != ruleRepoNameMissing.ID {
					t.Errorf("expected rule ID %s, got %s", ruleRepoNameMissing.ID, result.RuleMetadata.ID)
				}
			}
		})
	}
}
