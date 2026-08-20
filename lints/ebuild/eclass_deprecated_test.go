package ebuild

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arran4/g2"
)

func TestEclassDeprecatedLintRule(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Setup Master Repo
	masterDir := filepath.Join(tempDir, "master")
	masterEclassDir := filepath.Join(masterDir, "eclass")
	if err := os.MkdirAll(masterEclassDir, 0755); err != nil {
		t.Fatalf("Failed to create master eclass dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(masterEclassDir, "upstream-deprecated.eclass"), []byte("# @ECLASS: upstream-deprecated\n# @DEPRECATED: none\n"), 0644); err != nil {
		t.Fatalf("Failed to write upstream eclass: %v", err)
	}

	// 2. Setup repos.conf to map "gentoo" to our masterDir
	reposConfDir := filepath.Join(tempDir, "etc", "portage")
	if err := os.MkdirAll(reposConfDir, 0755); err != nil {
		t.Fatalf("Failed to create repos.conf dir: %v", err)
	}
	reposConfPath := filepath.Join(reposConfDir, "repos.conf")
	reposConfContent := "[gentoo]\nlocation = " + masterDir + "\n"
	if err := os.WriteFile(reposConfPath, []byte(reposConfContent), 0644); err != nil {
		t.Fatalf("Failed to write repos.conf: %v", err)
	}

    // Override defaults for testing
    DefaultReposConfPath = reposConfPath
    DefaultReposBasePath = tempDir

	site := &g2.SiteData{
		LayoutConf: &g2.LayoutConf{
            Entries: []g2.LayoutConfEntry{
                {Key: "masters", Value: "gentoo"},
            },
        },
		Eclasses: []*g2.Ebuild{
			{
				Path:    "eclass/deprecated-eclass.eclass",
				RawText: "# @ECLASS: deprecated-eclass\n# @DEPRECATED: replacement-eclass",
			},
			{
				Path:    "eclass/valid-eclass.eclass",
				RawText: "# @ECLASS: valid-eclass\n",
			},
		},
		Categories: []g2.CategoryData{
			{
				Packages: []g2.PackageData{
					{
						Category: "app-test",
						Name:     "testpkg1",
						Versions: []g2.VersionData{
							{
								Version: "1.0",
								Ebuild: &g2.Ebuild{
									Vars: map[string]string{
										"INHERITED": "deprecated-eclass upstream-deprecated",
									},
								},
							},
						},
					},
					{
						Category: "app-test",
						Name:     "testpkg2",
						Versions: []g2.VersionData{
							{
								Version: "1.0",
								Ebuild: &g2.Ebuild{
									Vars: map[string]string{
										"INHERITED": "valid-eclass",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name       string
		site       *g2.SiteData
		wantCount  int
		wantErrors []string
	}{
		{
			name:      "Mixed eclasses including upstream",
			site:      site,
			wantCount: 2,
			wantErrors: []string{
				"[Warning] Ebuild 1.0 inherits a deprecated eclass 'deprecated-eclass'.",
                "[Warning] Ebuild 1.0 inherits a deprecated eclass 'upstream-deprecated'.",
			},
		},
		{
			name:      "Empty site",
			site:      &g2.SiteData{},
			wantCount: 0,
		},
	}

	rule := &EclassDeprecatedLintRule{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := rule.LintRepo(".", tt.site)
			if len(results) != tt.wantCount {
				t.Errorf("expected %d results, got %d", tt.wantCount, len(results))
			}
			if tt.wantCount > 0 && len(results) == tt.wantCount {
                // Ensure all expected warnings exist
				for _, want := range tt.wantErrors {
                    found := false
                    for _, res := range results {
                        if res.Message == want {
                            found = true
                            break
                        }
                    }
					if !found {
						t.Errorf("expected result message %q not found", want)
					}
				}
			}
		})
	}
}
