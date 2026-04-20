package ebuild_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestLicenseSanityLintRule(t *testing.T) {
	rule := &ebuild.LicenseSanityLintRule{}

	tests := []struct {
		name    string
		license string
		want    int // number of warnings
	}{
		{"Valid License", "GPL-2", 0},
		{"Multiple Valid", "GPL-2 MIT", 0},
		{"Contains Slash", "GPL/2", 1},
		{"Missing Alphanum", "|| ( MIT )", 0},     // || and ( ) are stripped by parser, leaving MIT
		{"Invalid Alphanum Check", "|| ( - )", 1}, // No alphanum in license name "-"
		{"Full Text Like", "This is a custom license all rights reserved", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"LICENSE": tt.license,
							},
						},
					},
				},
			}
			warnings := rule.Lint(".", pkg)
			if len(warnings) != tt.want {
				t.Errorf("got %d warnings, want %d: %v", len(warnings), tt.want, warnings)
			}
		})
	}
}
