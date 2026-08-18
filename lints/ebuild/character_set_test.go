package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestCharacterSetLintRule(t *testing.T) {
	rule := &CharacterSetLintRule{}

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
					RawText: string([]byte{0xff, 0xfe, 0xfd}), // Invalid UTF-8
				},
			},
		},
	}

	results := rule.Lint("", pkg)

	if len(results) != 1 {
		t.Fatalf("expected 1 results, got %d", len(results))
	}

	for _, result := range results {
		if result.RuleMetadata.ID != "CharacterSet" {
			t.Errorf("expected rule ID CharacterSet, got %s", result.RuleMetadata.ID)
		}
	}
}
