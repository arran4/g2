package ebuild_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestKeywordsSingleLineLintRule(t *testing.T) {
	rule := &ebuild.KeywordsSingleLineLintRule{}

	tests := []struct {
		name     string
		category string
		rawText  string
		want     int
	}{
		{"Valid KEYWORDS", "app-misc", "KEYWORDS=\"~amd64 ~x86\"", 0},
		{"KEYWORDS on multiple lines", "app-misc", "KEYWORDS=\"~amd64\n~x86\"", 1},
		{"KEYWORDS with variable", "app-misc", "KEYWORDS=\"~amd64 ${MY_KW}\"", 1},
		{"KEYWORDS append", "app-misc", "KEYWORDS=\"~amd64\"\nKEYWORDS+=\" ~x86\"", 2}, // Appending and multiple definitions
		{"KEYWORDS multiple defs", "app-misc", "KEYWORDS=\"~amd64\"\nKEYWORDS=\"~x86\"", 1},
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
