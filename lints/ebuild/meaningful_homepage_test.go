package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestMeaningfulHomepageLintRule(t *testing.T) {
	tests := []struct {
		name     string
		category string
		homepage string
		expected int
	}{
		{"Valid HOMEPAGE", "app-misc", "https://example.com", 0},
		{"Generic Gentoo HOMEPAGE", "app-misc", "https://www.gentoo.org/", 1},
		{"Generic Gentoo HOMEPAGE no trailing slash", "app-misc", "https://www.gentoo.org", 1},
		{"Valid and generic HOMEPAGE", "app-misc", "https://example.com https://www.gentoo.org/", 1},
	}

	rule := &MeaningfulHomepageLintRule{}

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
								"HOMEPAGE": tt.homepage,
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
