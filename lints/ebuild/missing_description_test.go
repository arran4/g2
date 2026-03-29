package ebuild_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestMissingDescriptionLintRule(t *testing.T) {
	rule := &ebuild.MissingDescriptionLintRule{}

	tests := []struct {
		name string
		desc string
		want int
	}{
		{"Valid description", "A very useful package that does things", 0},
		{"Missing description", "", 1},
		{"Short description", "Too short", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"DESCRIPTION": tt.desc,
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
