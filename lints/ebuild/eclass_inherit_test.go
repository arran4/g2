package ebuild

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/stretchr/testify/assert"
)

func TestConditionalInheritLintRule(t *testing.T) {
	rule := &ConditionalInheritLintRule{}

	tests := []struct {
		name          string
		ebuildContent string
		expected      int
	}{
		{
			name: "Unconditional inherit",
			ebuildContent: `
DESCRIPTION="A test ebuild"
inherit foo
`,
			expected: 0,
		},
		{
			name: "Conditional inherit on PV",
			ebuildContent: `
DESCRIPTION="A test ebuild"

if [[ ${PV} == *9999 ]]; then
	inherit git-r3
fi
`,
			expected: 0,
		},
		{
			name: "Conditional inherit on other var",
			ebuildContent: `
DESCRIPTION="A test ebuild"

if [[ ${USE_GIT} == "yes" ]]; then
	inherit git-r3
fi
`,
			expected: 1,
		},
		{
			name: "Inherit in function",
			ebuildContent: `
DESCRIPTION="A test ebuild"

src_prepare() {
	inherit foo
}
`,
			expected: 1,
		},
		{
			name: "Inherit in case clause on PV",
			ebuildContent: `
DESCRIPTION="A test ebuild"

case ${PV} in
	9999)
		inherit git-r3
		;;
esac
`,
			expected: 0,
		},
		{
			name: "Inherit in case clause on other var",
			ebuildContent: `
DESCRIPTION="A test ebuild"

case ${MY_VAR} in
	yes)
		inherit git-r3
		;;
esac
`,
			expected: 1,
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

			results := rule.Lint("", pkgData, nil)
			assert.Len(t, results, tt.expected)
		})
	}
}
