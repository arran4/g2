package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestVersionedUseLintRule(t *testing.T) {
	rule := &VersionedUseLintRule{}

	tests := []struct {
		name     string
		iuse     string
		expected int // Number of expected lint results
		qa       *g2.QAPolicy
	}{
		{
			name:     "No versioned flags",
			iuse:     "test doc",
			expected: 0,
		},
		{
			name:     "Flat explicitly versioned flags (qt4, qt5) without unversioned flag",
			iuse:     "qt4 qt5",
			expected: 0,
		},
		{
			name:     "Hierarchical versioned flags (gtk, gtk2)",
			iuse:     "gtk gtk2 doc",
			expected: 1, // Will find gtk and gtk2
		},
		{
			name:     "Hierarchical versioned flags (python, python3) and + flags",
			iuse:     "+python python3",
			expected: 1, // ParseIUSE handles '+' prefix, unversioned flag is 'python'
		},
		{
			name:     "Ignored via QA policy",
			iuse:     "gtk gtk2",
			expected: 0,
			qa: &g2.QAPolicy{
				Policies: map[string]string{
					"PG0801": "ignore",
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
