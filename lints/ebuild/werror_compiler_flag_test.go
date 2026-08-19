package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestWerrorCompilerFlagLintRule(t *testing.T) {
	rule := &WerrorCompilerFlagLintRule{}

	tests := []struct {
		name     string
		ebuild   string
		expected int
	}{
		{
			name: "No -Werror",
			ebuild: `src_compile() {
	append-flags -O2
	emake
}
`,
			expected: 0,
		},
		{
			name: "Append -Werror",
			ebuild: `src_compile() {
	append-flags -Werror
	emake
}
`,
			expected: 1,
		},
		{
			name: "Append -Werror to cflags",
			ebuild: `src_compile() {
	append-cflags -Werror
	emake
}
`,
			expected: 1,
		},
		{
			name: "Append -Werror to cxxflags",
			ebuild: `src_compile() {
	append-cxxflags -Werror
	emake
}
`,
			expected: 1,
		},
		{
			name: "Append -Werror to ldflags",
			ebuild: `src_compile() {
	append-ldflags -Werror
	emake
}
`,
			expected: 1,
		},
		{
			name: "Specific -Werror=...",
			ebuild: `src_compile() {
	append-flags -Werror=implicit-function-declaration
	emake
}
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

			results := rule.Lint("", pkgData)
			if len(results) != tt.expected {
				t.Errorf("Expected %d results, got %d", tt.expected, len(results))
			}
		})
	}
}
