package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestUseGuiLintRule(t *testing.T) {
	rule := &UseGuiLintRule{}

	tests := []struct {
		name     string
		iuse     string
		expected int // Number of expected lint results
		qa       *g2.QAPolicy
	}{
		{
			name:     "No gui or toolkit flags",
			iuse:     "test doc",
			expected: 0,
		},
		{
			name:     "Has gui flag and toolkit flag",
			iuse:     "gui gtk",
			expected: 0,
		},
		{
			name:     "Has toolkit flag (X) but no gui flag",
			iuse:     "X doc",
			expected: 1,
		},
		{
			name:     "Has multiple toolkit flags (gtk, qt5) but no gui flag",
			iuse:     "+gtk qt5 -X",
			expected: 1, // Only one warning should be emitted per ebuild
		},
		{
			name:     "Ignored via QA policy",
			iuse:     "gtk qt5",
			expected: 0,
			qa: &g2.QAPolicy{
				Policies: map[string]string{
					"PG0802": "ignore",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: "app-test",
				Name:     "foo",
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

			results := rule.LintWithQA("", pkg, tt.qa)
			if len(results) != tt.expected {
				t.Errorf("Expected %d results, got %d", tt.expected, len(results))
				for _, r := range results {
					t.Errorf("Message: %s", r.Message)
				}
			}
		})
	}
}
