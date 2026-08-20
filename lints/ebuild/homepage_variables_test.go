package ebuild_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestHomepageVariablesLintRule(t *testing.T) {
	rule := &ebuild.HomepageVariablesLintRule{}

	tests := []struct {
		name     string
		category string
		rawText  string
		want     int
	}{
		{"Valid HOMEPAGE", "app-misc", "HOMEPAGE=\"https://example.com/\"", 0},
		{"Variable in HOMEPAGE", "app-misc", "HOMEPAGE=\"https://example.com/${PN}\"", 1},
		{"Multiple variables in HOMEPAGE", "app-misc", "HOMEPAGE=\"https://example.com/${PN}/${PV}\"", 2},
		{"Short variable in HOMEPAGE", "app-misc", "HOMEPAGE=\"https://example.com/$PN\"", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: tt.category,
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							RawText: tt.rawText,
						},
					},
				},
			}
			warnings := rule.Lint(".", pkg)
			if len(warnings) != tt.want {
				t.Errorf("got %d warnings, want %d", len(warnings), tt.want)
			}
		})
	}
}
