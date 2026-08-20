package ebuild_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestSrcUriHomepageLintRule(t *testing.T) {
	rule := &ebuild.SrcUriHomepageLintRule{}

	tests := []struct {
		name     string
		category string
		rawText  string
		want     int
	}{
		{"Valid SRC_URI", "app-misc", "SRC_URI=\"https://example.com/foo.tar.gz\"", 0},
		{"SRC_URI refers to HOMEPAGE", "app-misc", "SRC_URI=\"${HOMEPAGE}/foo.tar.gz\"", 1},
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
