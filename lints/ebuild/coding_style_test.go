package ebuild_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestCodingStyleLintRule(t *testing.T) {
	rule := &ebuild.CodingStyleLintRule{}

	tests := []struct {
		name     string
		category string
		rawText  string
		want     int
	}{
		{"Valid ebuild", "app-misc", "pkg_setup() {\n[[ -n \"${foo}\" ]] || die\n}", 0},
		{"POSIX condition [", "app-misc", "pkg_setup() {\n[ -n \"${foo}\" ] || die\n}", 1},
		{"POSIX condition test", "app-misc", "pkg_setup() {\ntest -n \"${foo}\" || die\n}", 1},
		{"Unbracketed variable", "app-misc", "pkg_setup() {\necho $foo\n}", 1},
		{"Special unbracketed variable", "app-misc", "pkg_setup() {\necho $1 $@ $* $? $$ $! $- $#\n}", 0},
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
