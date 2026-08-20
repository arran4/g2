package ebuild

import (
	"strings"
	"testing"

	"github.com/arran4/g2"
)

func TestInstallationPathsLintRule(t *testing.T) {
	rule := &InstallationPathsLintRule{}

	tests := []struct {
		name          string
		ebuildContent string
		expected      int
		qaPolicy      *g2.QAPolicy
	}{
		{
			name: "Allowed top level",
			ebuildContent: `
src_install() {
	insinto /opt/package
}
`,
			expected: 0,
		},
		{
			name: "Violation - unallowed top level",
			ebuildContent: `
src_install() {
	insinto /mycustomdir
}
`,
			expected: 1,
		},
		{
			name: "Violation - subdir in /bin",
			ebuildContent: `
src_install() {
	insinto /bin/subdir
}
`,
			expected: 1,
		},
		{
			name: "Violation - subdir in /usr/bin",
			ebuildContent: `
src_install() {
	insinto /usr/bin/subdir
}
`,
			expected: 1,
		},
		{
			name: "Violation - subdir in /sbin",
			ebuildContent: `
src_install() {
	insinto /sbin/subdir
}
`,
			expected: 1,
		},
		{
			name: "Violation - subdir in /usr/sbin",
			ebuildContent: `
src_install() {
	insinto /usr/sbin/subdir
}
`,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: "app-misc",
				Name:     "testapp",
				Versions: []g2.VersionData{
					{
						Ebuild: &g2.Ebuild{
							Path:    "app-misc/testapp/testapp-1.0.ebuild",
							RawText: strings.TrimSpace(tt.ebuildContent),
						},
					},
				},
			}

			results := rule.LintWithQA("", pkg, tt.qaPolicy)

			if len(results) != tt.expected {
				t.Errorf("Expected %d results, got %d", tt.expected, len(results))
				for _, r := range results {
					t.Logf("Result: %s", r.Message)
				}
			}
		})
	}
}
