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
