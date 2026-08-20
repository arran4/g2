package ebuild

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/stretchr/testify/assert"
)

func TestDeprecatedInsintoLintRule(t *testing.T) {
	rule := &DeprecatedInsintoLintRule{}

	tests := []struct {
		name          string
		ebuildContent string
		expected      int
		qaPolicy      *g2.QAPolicy
	}{
		{
			name: "Valid insinto usage",
			ebuildContent: `
src_install() {
	insinto /usr/share/doc/${PF}
	doins README.md
}
`,
			expected: 0,
		},
		{
			name: "Deprecated insinto /etc/init.d",
			ebuildContent: `
src_install() {
	insinto /etc/init.d
	doins my-init-script
}
`,
			expected: 1,
		},
		{
			name: "Deprecated exeinto /usr/lib/systemd/system",
			ebuildContent: `
src_install() {
	exeinto /usr/lib/systemd/system
	doexe myservice.service
}
`,
			expected: 1,
		},
		{
			name: "Deprecated insinto with ignored QA policy",
			ebuildContent: `
src_install() {
	insinto /etc/init.d
	doins my-init-script
}
`,
			expected: 0,
			qaPolicy: &g2.QAPolicy{
				Policies: map[string]string{
					"PG0805": "ignore",
				},
			},
		},
		{
			name: "Deprecated insinto with string concat",
			ebuildContent: `
src_install() {
	insinto "/usr/lib/systemd/system"
	doins my-init-script
}
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

			results := rule.LintWithQA("", pkgData, tt.qaPolicy, nil)
			assert.Len(t, results, tt.expected)
		})
	}
}
