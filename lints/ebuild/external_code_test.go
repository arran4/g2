package ebuild_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestExternalCodeLintRule(t *testing.T) {
	rule := &ebuild.ExternalCodeLintRule{}

	tests := []struct {
		name     string
		category string
		rawText  string
		want     int
	}{
		{"Valid ebuild", "app-misc", "pkg_setup() {\necho \"test\"\n}", 0},
		{"Source external file", "app-misc", "pkg_setup() {\nsource ./foo\n}", 1},
		{"Dot source external file", "app-misc", "pkg_setup() {\n. ./foo\n}", 1},
		{"Eval usage", "app-misc", "pkg_setup() {\neval \"echo test\"\n}", 1},
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
