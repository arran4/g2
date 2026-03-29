package metadata_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/metadata"
)

func TestMaintainerMissingLintRule(t *testing.T) {
	rule := &metadata.MaintainerLintRule{}

	t.Run("Missing maintainer", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Maintainers: []g2.Maintainer{},
			},
		}
		warnings := rule.Lint(".", pkg)
		if len(warnings) == 0 {
			t.Error("expected warning for missing maintainer, got none")
		}
	})

	t.Run("Has maintainer", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Maintainers: []g2.Maintainer{
					{Email: "dev@example.com"},
				},
			},
		}
		warnings := rule.Lint(".", pkg)
		if len(warnings) > 0 {
			t.Errorf("expected no warnings, got %v", warnings)
		}
	})
}
