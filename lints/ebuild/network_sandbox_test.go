package ebuild_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestNetworkSandboxLintRule(t *testing.T) {
	rule := &ebuild.NetworkSandboxLintRule{}

	tests := []struct {
		name     string
		category string
		version  string
		restrict string
		want     int
	}{
		{"No network-sandbox", "app-misc", "1.0", "test", 0},
		{"With network-sandbox", "app-misc", "1.0", "network-sandbox test", 1},
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
								"RESTRICT": tt.restrict,
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
