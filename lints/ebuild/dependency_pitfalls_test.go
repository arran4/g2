package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestDependencyPitfallsLintRule(t *testing.T) {
	rule := &DependencyPitfallsLintRule{}

	tests := []struct {
		name     string
		pkg      *g2.PackageData
		qaPolicy *g2.QAPolicy
		expected int // Number of expected errors
	}{
		{
			name: "Valid deps",
			pkg: &g2.PackageData{
				Category: "dev-libs",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"DEPEND":  "|| ( dev-libs/A dev-libs/B ) !!sys-libs/bar",
								"RDEPEND": "|| ( dev-libs/A dev-libs/B ) !app-misc/foo",
							},
						},
					},
				},
			},
			qaPolicy: nil,
			expected: 0,
		},
		{
			name: "Slot operator inside any-of group",
			pkg: &g2.PackageData{
				Category: "dev-libs",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"RDEPEND": "|| ( dev-libs/A:= dev-libs/B )",
							},
						},
					},
				},
			},
			qaPolicy: nil,
			expected: 1, // 1 warning for dev-libs/A:=
		},
		{
			name: "Weak blocker in DEPEND",
			pkg: &g2.PackageData{
				Category: "dev-libs",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"DEPEND": "!app-misc/foo",
							},
						},
					},
				},
			},
			qaPolicy: nil,
			expected: 1, // 1 warning
		},
		{
			name: "Nested any-of group",
			pkg: &g2.PackageData{
				Category: "dev-libs",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"RDEPEND": "a? ( || ( dev-libs/A:= dev-libs/B:= ) )",
							},
						},
					},
				},
			},
			qaPolicy: nil,
			expected: 2, // 2 warnings
		},
		{
			name: "Any-of inside another group",
			pkg: &g2.PackageData{
				Category: "dev-libs",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"DEPEND": "|| ( a? ( dev-libs/A:= ) )",
							},
						},
					},
				},
			},
			qaPolicy: nil,
			expected: 1, // 1 warning
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := rule.LintWithQA("", tc.pkg, tc.qaPolicy)

			if len(results) != tc.expected {
				t.Errorf("Expected %d results, got %d", tc.expected, len(results))
				for _, res := range results {
					t.Logf("Result: %s", res.Message)
				}
			}
		})
	}
}
