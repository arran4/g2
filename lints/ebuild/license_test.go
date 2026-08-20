package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestLicenseLintRule(t *testing.T) {
	tests := []struct {
		name     string
		category string
		license  string
		expected int
	}{
		{"Has license", "app-misc", "GPL-2", 0},
		{"No license", "app-misc", "", 1},
		{"Virtual with no license", "virtual", "", 0},
	}

	rule := &LicenseLintRule{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: tt.category,
				Name:     "example",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"LICENSE": tt.license,
							},
						},
					},
				},
			}

			results := rule.Lint("", pkg)

			if len(results) != tt.expected {
				t.Errorf("Expected %d results, got %d", tt.expected, len(results))
			}
		})
	}
}

func TestLicenseExistsRepoLintRule(t *testing.T) {
	rule := &LicenseExistsRepoLintRule{}

	site := &g2.SiteData{
		ProvidedLicenses: []string{"GPL-2", "MIT"},
		LicenseMapping: map[string][]string{
			"FREE": {"GPL-2", "MIT"},
		},
		Categories: []g2.CategoryData{
			{
				Name: "app-misc",
				Packages: []g2.PackageData{
					{
						Category: "app-misc",
						Name:     "example1",
						Versions: []g2.VersionData{
							{
								Version: "1.0",
								Ebuild: &g2.Ebuild{
									Vars: map[string]string{
										"LICENSE": "GPL-2",
									},
								},
							},
						},
					},
					{
						Category: "app-misc",
						Name:     "example2",
						Versions: []g2.VersionData{
							{
								Version: "1.0",
								Ebuild: &g2.Ebuild{
									Vars: map[string]string{
										"LICENSE": "UNKNOWN",
									},
								},
							},
						},
					},
					{
						Category: "app-misc",
						Name:     "example3",
						Versions: []g2.VersionData{
							{
								Version: "1.0",
								Ebuild: &g2.Ebuild{
									Vars: map[string]string{
										"LICENSE": "FREE",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	results := rule.LintRepo("", site)

	if len(results) != 1 {
		t.Errorf("Expected 1 result for UNKNOWN license, got %d", len(results))
		for _, res := range results {
			t.Logf("Result: %v", res.Message)
		}
	} else if results[0].Package != "app-misc/example2" {
		t.Errorf("Expected result for app-misc/example2, got %v", results[0].Package)
	}
}
