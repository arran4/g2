package ebuild

import (
	"strings"
	"testing"

	"github.com/arran4/g2"
)

func TestGameInstallLocationsLintRule(t *testing.T) {
	rule := &GameInstallLocationsLintRule{}

	tests := []struct {
		name          string
		ebuildContent string
		expected      int
		qaPolicy      *g2.QAPolicy
	}{
		{
			name: "No violation",
			ebuildContent: `
src_install() {
	insinto /usr/share/games
}
`,
			expected: 0,
		},
		{
			name: "Violation /usr/games",
			ebuildContent: `
src_install() {
	insinto /usr/games/bin
}
`,
			expected: 1,
		},
		{
			name: "Violation /etc/games",
			ebuildContent: `
src_install() {
	insinto /etc/games/config
}
`,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: "games-action",
				Name:     "testgame",
				Versions: []g2.VersionData{
					{
						Ebuild: &g2.Ebuild{
							Path:    "games-action/testgame/testgame-1.0.ebuild",
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
