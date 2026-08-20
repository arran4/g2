package ebuild

import (
	"strings"
	"testing"

	"github.com/arran4/g2"
)

func TestAbsoluteSymlinkLintRule(t *testing.T) {
	rule := &AbsoluteSymlinkLintRule{}

	tests := []struct {
		name          string
		ebuildContent string
		expected      int
		qaPolicy      *g2.QAPolicy
	}{
		{
			name: "No violation relative",
			ebuildContent: `
src_install() {
	dosym ../lib/frobnicate/frobnicate /usr/bin/frobnicate
}
`,
			expected: 0,
		},
		{
			name: "No violation /proc exception",
			ebuildContent: `
src_install() {
	dosym /proc/self/mounts /etc/mtab
}
`,
			expected: 0,
		},
		{
			name: "Violation absolute path",
			ebuildContent: `
src_install() {
	dosym /usr/lib/frobnicate/frobnicate /usr/bin/frobnicate
}
`,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: "sys-apps",
				Name:     "testapp",
				Versions: []g2.VersionData{
					{
						Ebuild: &g2.Ebuild{
							Path:    "sys-apps/testapp/testapp-1.0.ebuild",
							RawText: strings.TrimSpace(tt.ebuildContent),
						},
					},
				},
			}

			results := rule.LintWithQA("", pkg, tt.qaPolicy, nil)

			if len(results) != tt.expected {
				t.Errorf("Expected %d results, got %d", tt.expected, len(results))
				for _, r := range results {
					t.Logf("Result: %s", r.Message)
				}
			}
		})
	}
}
