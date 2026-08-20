package ebuild

import (
	"strings"
	"testing"

	"github.com/arran4/g2"
)

func TestStaticLibrariesLintRule(t *testing.T) {
	rule := &StaticLibrariesLintRule{}

	tests := []struct {
		name          string
		ebuildContent string
		expected      int
		qaPolicy      *g2.QAPolicy
	}{
		{
			name: "No violation - dynamic lib",
			ebuildContent: `
src_install() {
	into /lib
	dolib libtest.so
}
`,
			expected: 0,
		},
		{
			name: "No violation - static lib in /usr/lib",
			ebuildContent: `
src_install() {
	into /usr/lib
	dolib.a libtest.a
}
`,
			expected: 0,
		},
		{
			name: "Violation - static lib in /lib",
			ebuildContent: `
src_install() {
	into /lib
	dolib.a libtest.a
}
`,
			expected: 1,
		},
		{
			name: "Violation - libtool file in /lib",
			ebuildContent: `
src_install() {
	insinto /lib
	doins libtest.la
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
