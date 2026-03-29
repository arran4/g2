package metadata

import (
	"testing"

	"github.com/arran4/g2"
)

func TestMaintainerLintRule(t *testing.T) {
	rule := &MaintainerLintRule{}

	t.Run("Has maintainer", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Maintainers: []g2.Maintainer{{Email: "test@example.com"}},
			},
		}
		warnings := rule.Lint("", pkg)
		if len(warnings) != 0 {
			t.Errorf("Expected 0 warnings, got %v", warnings)
		}
	})

	t.Run("Missing maintainer", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Maintainers: []g2.Maintainer{},
			},
		}
		warnings := rule.Lint("", pkg)
		if len(warnings) != 1 {
			t.Errorf("Expected 1 warning, got %v", warnings)
		} else if warnings[0] != "[Warning] metadata.xml is missing a maintainer. Add at least one <maintainer> element." {
			t.Errorf("Unexpected warning: %s", warnings[0])
		}
	})

	t.Run("Missing metadata entirely", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: nil,
		}
		warnings := rule.Lint("", pkg)
		if len(warnings) != 0 {
			t.Errorf("Expected 0 warnings, got %v", warnings)
		}
	})
}
