package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestPkgConfigDirectCallLintRule(t *testing.T) {
	rule := &PkgConfigDirectCallLintRule{}

	tests := []struct {
		name     string
		ebuild   string
		expected int
	}{
		{
			name: "Direct call",
			ebuild: `src_compile() {
	pkg-config --libs ncurses
	emake
}
`,
			expected: 1,
		},
		{
			name: "tc-getPKG_CONFIG call",
			ebuild: `src_compile() {
	$(tc-getPKG_CONFIG) --libs ncurses
	emake
}
`,
			expected: 0,
		},
		{
			name: "Direct call in subshell",
			ebuild: `src_compile() {
	sed -i -e "s:-lncurses:$(pkg-config --libs ncurses):"
	emake
}
`,
			expected: 1,
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
