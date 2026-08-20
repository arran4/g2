package ebuild

import (
	"strings"
	"testing"

	"github.com/arran4/g2"
)

func TestStrictMultilibLayoutLintRule(t *testing.T) {
	rule := &StrictMultilibLayoutLintRule{}

	tests := []struct {
		name          string
		ebuildContent string
		expected      int
		qaPolicy      *g2.QAPolicy
	}{
		{
			name: "No violation lib64",
			ebuildContent: `
src_install() {
	insinto /usr/lib64
}
`,
			expected: 0,
		},
		{
			name: "No violation subfolder",
			ebuildContent: `
src_install() {
	insinto /usr/lib/package
}
`,
			expected: 0,
		},
		{
			name: "Violation /lib",
			ebuildContent: `
src_install() {
	into /lib
}
`,
			expected: 1,
		},
		{
			name: "Violation /usr/lib",
			ebuildContent: `
src_install() {
	into /usr/lib
}
`,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: "sys-libs",
				Name:     "testlib",
				Versions: []g2.VersionData{
					{
						Ebuild: &g2.Ebuild{
							Path:    "sys-libs/testlib/testlib-1.0.ebuild",
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
