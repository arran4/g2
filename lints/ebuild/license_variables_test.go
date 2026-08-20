package ebuild_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestLicenseVariablesLintRule(t *testing.T) {
	rule := &ebuild.LicenseVariablesLintRule{}

	tests := []struct {
		name     string
		category string
		rawText  string
		want     int
	}{
		{"Valid LICENSE", "app-misc", "LICENSE=\"GPL-2\"", 0},
		{"LICENSE append", "app-misc", "LICENSE=\"${LICENSE} GPL-2\"", 0},
		{"LICENSE append +=", "app-misc", "LICENSE+=\" GPL-2\"", 0},
		{"Variable in LICENSE", "app-misc", "LICENSE=\"${MY_LIC}\"", 1},
		{"Short Variable in LICENSE", "app-misc", "LICENSE=\"$MY_LIC\"", 1},
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
			warnings := rule.Lint(".", pkg, nil)
			if len(warnings) != tt.want {
				t.Errorf("got %d warnings, want %d", len(warnings), tt.want)
			}
		})
	}
}
