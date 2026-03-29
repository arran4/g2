package ebuild_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestMissingHomepageLintRule(t *testing.T) {
	rule := &ebuild.MissingHomepageLintRule{}

	tests := []struct {
		name     string
		category string
		homepage string
		want     int
	}{
		{"Valid homepage", "app-misc", "https://example.com", 0},
		{"Missing homepage", "app-misc", "", 1},
		{"Invalid schema", "app-misc", "git://example.com", 1},
		{"Exempt virtual", "virtual", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: tt.category,
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"HOMEPAGE": tt.homepage,
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
