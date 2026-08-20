package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestRedefinedVariablesLintRule(t *testing.T) {
	rule := &RedefinedVariablesLintRule{}

	tests := []struct {
		name     string
		ebuild   string
		expected int
	}{
		{
			name: "Valid variables",
			ebuild: `MY_P="${P/pyopenal/PyOpenAL}"
SRC_URI="http://download.gna.org/pyopenal/${MY_P}.tar.gz"
S=${WORKDIR}/${MY_P}
`,
			expected: 0,
		},
		{
			name: "Redefined P",
			ebuild: `P="PyOpenAL-1.0"
`,
			expected: 1,
		},
		{
			name: "Redefined PV",
			ebuild: `PV="1.0"
`,
			expected: 1,
		},
		{
			name: "Redefined PN",
			ebuild: `PN="PyOpenAL"
`,
			expected: 1,
		},
		{
			name: "Redefined PF",
			ebuild: `PF="PyOpenAL-1.0-r1"
`,
			expected: 1,
		},
		{
			name: "Redefined PR",
			ebuild: `PR="r1"
`,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgData := &g2.PackageData{
				Category: "dev-python",
				Name:     "pyopenal",
				Versions: []g2.VersionData{
					{
						Ebuild: &g2.Ebuild{
							Path:    "dev-python/pyopenal/pyopenal-1.0.ebuild",
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
