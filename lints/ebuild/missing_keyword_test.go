package ebuild_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestMissingKeywordLintRule(t *testing.T) {
	rule := &ebuild.MissingKeywordLintRule{}

	tests := []struct {
		name     string
		category string
		version  string
		keywords string
		want     int
	}{
		{"Has keywords", "app-misc", "1.0", "~amd64 x86", 0},
		{"Missing keywords", "app-misc", "1.0", "", 1},
		{"Live package", "app-misc", "9999", "", 0},
		{"Virtual package", "virtual", "1.0", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: tt.category,
				Versions: []g2.VersionData{
					{
						Version: tt.version,
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"KEYWORDS": tt.keywords,
							},
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
