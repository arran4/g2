package ebuild

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/stretchr/testify/assert"
)

func TestGlobalScopeLintRule(t *testing.T) {
	rule := &GlobalScopeLintRule{}

	tests := []struct {
		name          string
		ebuildContent string
		expected      int
	}{
		{
			name: "Clean ebuild",
			ebuildContent: `
DESCRIPTION="A test ebuild"
HOMEPAGE="https://example.com"
LICENSE="GPL-2"
SLOT="0"

src_prepare() {
	sed -i 's/a/b/' file
	has_version a/b
}
`,
			expected: 0,
		},
		{
			name: "sed in global scope",
			ebuildContent: `
DESCRIPTION="A test ebuild"

sed -i 's/a/b/' file

src_prepare() {
	true
}
`,
			expected: 1,
		},
		{
			name: "has_version in subshell in global scope",
			ebuildContent: `
DESCRIPTION="A test ebuild"

FOO=$(has_version a/b)
`,
			expected: 1,
		},
		{
			name: "multiple bad commands",
			ebuildContent: `
DESCRIPTION="A test ebuild"

awk '{print $1}' file
grep "foo" file
`,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgData := &g2.PackageData{
				Category: "app-test",
				Name:     "testpkg",
				Versions: []g2.VersionData{
					{
						Ebuild: &g2.Ebuild{
							Path:    "testpkg-1.0.ebuild",
							RawText: tt.ebuildContent,
						},
					},
				},
			}

			results := rule.Lint("", pkgData)
			assert.Len(t, results, tt.expected)
		})
	}
}
