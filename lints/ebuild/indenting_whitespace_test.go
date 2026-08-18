package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestIndentingWhitespaceLintRule(t *testing.T) {
	rule := &IndentingWhitespaceLintRule{}

	pkg := &g2.PackageData{
		Category: "app-misc",
		Name:     "test",
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				Ebuild: &g2.Ebuild{
					RawText: "src_prepare() {\n\techo \"hello\"\n}\n",
				},
			},
			{
				Version: "2.0",
				Ebuild: &g2.Ebuild{
					RawText: "src_prepare() {\n  echo \"hello\"\n}\n", // leading space
				},
			},
			{
				Version: "3.0",
				Ebuild: &g2.Ebuild{
					RawText: "src_prepare() { \n\techo \"hello\"\n}\n", // trailing space
				},
			},
		},
	}

	results := rule.Lint("", pkg)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for _, result := range results {
		if result.RuleMetadata.ID != "IndentingWhitespace" {
			t.Errorf("expected rule ID IndentingWhitespace, got %s", result.RuleMetadata.ID)
		}
	}
}
