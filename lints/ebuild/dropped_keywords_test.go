package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestDroppedKeywordsLintRule(t *testing.T) {
	rule := &DroppedKeywordsLintRule{}

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
			name:     "No dropped keywords",
			category: "app-misc",
			pkgName:  "testpkg",
			versions: []struct {
				version  string
				keywords string
			}{
				{"1.0", "~amd64 ~x86"},
				{"1.1", "~amd64 ~x86"},
				{"1.2", "amd64 ~x86"},
			},
			expected: 0,
		},
		{
			name:     "Dropped keyword x86",
			category: "app-misc",
			pkgName:  "testpkg",
			versions: []struct {
				version  string
				keywords string
			}{
				{"1.0", "~amd64 ~x86"},
				{"1.1", "~amd64"},
			},
			expected: 1,
		},
		{
			name:     "Dropped keyword restored",
			category: "app-misc",
			pkgName:  "testpkg",
			versions: []struct {
				version  string
				keywords string
			}{
				{"1.0", "~amd64 ~x86"},
				{"1.1", "~amd64"},
				{"1.2", "~amd64 ~x86"},
			},
			expected: 0,
		},
		{
			name:     "Disabled keyword does not drop",
			category: "app-misc",
			pkgName:  "testpkg",
			versions: []struct {
				version  string
				keywords string
			}{
				{"1.0", "~amd64 ~x86"},
				{"1.1", "~amd64 -x86"},
			},
			expected: 0,
		},
		{
			name:     "Star keyword overrides",
			category: "app-misc",
			pkgName:  "testpkg",
			versions: []struct {
				version  string
				keywords string
			}{
				{"1.0", "~amd64 ~x86"},
				{"1.1", "-* amd64"},
			},
			expected: 0,
		},
		{
			name:     "Only one version",
			category: "app-misc",
			pkgName:  "testpkg",
			versions: []struct {
				version  string
				keywords string
			}{
				{"1.0", "~amd64 ~x86"},
			},
			expected: 0,
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

			results := rule.Lint("", pkg, nil)
			if len(results) != tt.expected {
				t.Errorf("expected %d results, got %d", tt.expected, len(results))
				for _, r := range results {
					t.Logf("Result: %s", r.Message)
				}
			}
		})
	}
}
