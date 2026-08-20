package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestRedundantSLintRule(t *testing.T) {
	rule := &RedundantSLintRule{}

	tests := []struct {
		name     string
		ebuild   string
		expected int
	}{
		{
			name: "Redundant S",
			ebuild: `S="${WORKDIR}/${P}"
`,
			expected: 1,
		},
		{
			name: "Redundant S without quotes",
			ebuild: `S=${WORKDIR}/${P}
`,
			expected: 1,
		},
		{
			name: "Redundant S with $P",
			ebuild: `S="${WORKDIR}/$P"
`,
			expected: 1,
		},
		{
			name: "Not redundant S",
			ebuild: `S="${WORKDIR}/${MY_P}"
`,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgData := &g2.PackageData{
				Category: "dev-libs",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Ebuild: &g2.Ebuild{
							Path:    "dev-libs/foo/foo-1.0.ebuild",
							RawText: tt.ebuild,
						},
					},
				},
			}

			results := rule.Lint("", pkgData, nil)
			if len(results) != tt.expected {
				t.Errorf("Expected %d results, got %d", tt.expected, len(results))
			}
		})
	}
}
