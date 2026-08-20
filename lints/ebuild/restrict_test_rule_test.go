package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestRestrictTestLintRule(t *testing.T) {
	tests := []struct {
		name     string
		iuse     string
		restrict string
		expected int
	}{
		{"No test in IUSE", "doc", "", 0},
		{"Test in IUSE, explicit restrict", "test", "!test? ( test )", 0},
		{"Test in IUSE, unconditional restrict", "test", "test", 0},
		{"Test in IUSE, missing restrict", "test", "", 1},
		{"Test in IUSE, wrong restrict condition", "test", "foo? ( test )", 1},
	}

	rule := &RestrictTestLintRule{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: "app-misc",
				Name:     "example",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"IUSE":     tt.iuse,
								"RESTRICT": tt.restrict,
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
