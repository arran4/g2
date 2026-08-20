package ebuild_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestDVariablesLintRule(t *testing.T) {
	rule := &ebuild.DVariablesLintRule{}

	tests := []struct {
		name     string
		category string
		rawText  string
		want     int
	}{
		{"Valid usage in src_install", "app-misc", "src_install() {\necho ${D}\n}", 0},
		{"Valid usage in pkg_preinst", "app-misc", "pkg_preinst() {\necho ${ED}\n}", 0},
		{"Invalid usage in src_configure", "app-misc", "src_configure() {\necho ${D}\n}", 1},
		{"Invalid usage in global scope", "app-misc", "MY_VAR=\"${D}/foo\"", 1},
		{"Valid short var usage", "app-misc", "src_install() {\necho $D\n}", 0},
		{"Invalid short var usage in global scope", "app-misc", "MY_VAR=\"$D/foo\"", 1},
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
