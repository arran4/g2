package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestUnstableOnlyLintRule(t *testing.T) {
	rule := &UnstableOnlyLintRule{}

	tests := []struct {
		name     string
		category string
		pkgName  string
		versions []struct {
			version  string
			keywords string
		}
		expected int
	}{
		{
			name:     "Has stable",
			category: "app-misc",
			pkgName:  "testpkg",
			versions: []struct {
				version  string
				keywords string
			}{
				{"1.0", "amd64 ~x86"},
				{"1.1", "~amd64 ~x86"},
			},
			expected: 1, // x86 is unstable only
		},
		{
			name:     "All stable",
			category: "app-misc",
			pkgName:  "testpkg",
			versions: []struct {
				version  string
				keywords string
			}{
				{"1.0", "amd64 x86"},
			},
			expected: 0,
		},
		{
			name:     "Unstable only",
			category: "app-misc",
			pkgName:  "testpkg",
			versions: []struct {
				version  string
				keywords string
			}{
				{"1.0", "~amd64 ~x86"},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: tt.category,
				Name:     tt.pkgName,
				Versions: make([]g2.VersionData, 0, len(tt.versions)),
			}

			for _, v := range tt.versions {
				pkg.Versions = append(pkg.Versions, g2.VersionData{
					Version: v.version,
					Ebuild: &g2.Ebuild{
						Vars: map[string]string{
							"KEYWORDS": v.keywords,
						},
					},
				})
			}

			results := rule.Lint("", pkg)
			if len(results) != tt.expected {
				t.Errorf("expected %d results, got %d", tt.expected, len(results))
				for _, r := range results {
					t.Logf("Result: %s", r.Message)
				}
			}
		})
	}
}
