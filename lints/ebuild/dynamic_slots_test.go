package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestDynamicSlotsLintRule(t *testing.T) {
	tests := []struct {
		name     string
		category string
		iuse     string
		expected int
	}{
		{"Valid no multislot", "sys-devel", "", 0},
		{"Invalid multislot in IUSE", "sys-devel", "multislot", 1},
		{"Invalid +multislot in IUSE", "sys-devel", "+multislot", 1},
		{"Invalid -multislot in IUSE", "sys-devel", "test -multislot", 1},
	}

	rule := &DynamicSlotsLintRule{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: tt.category,
				Name:     "gcc",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"IUSE": tt.iuse,
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
