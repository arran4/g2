package metadata_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/metadata"
)

func TestMaintainerNeededLintRule(t *testing.T) {
	rule := &metadata.MaintainerNeededLintRule{}

	t.Run("Missing maintainer and missing comment", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Maintainers: []g2.Maintainer{},
				Comments:    []string{},
			},
		}
		warnings := rule.Lint(".", pkg, nil)
		if len(warnings) == 0 {
			t.Error("expected warning for missing maintainer-needed comment, got none")
		} else if warnings[0].RuleMetadata.ID != "MaintainerNeeded" {
			t.Errorf("expected MaintainerNeeded rule, got %s", warnings[0].RuleMetadata.ID)
		}
	})

	t.Run("Missing maintainer and has comment", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Maintainers: []g2.Maintainer{},
				Comments:    []string{" maintainer-needed "},
			},
		}
		warnings := rule.Lint(".", pkg, nil)
		if len(warnings) > 0 {
			t.Errorf("expected no warnings, got %v", warnings)
		}
	})

	t.Run("Has maintainer and has comment", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Maintainers: []g2.Maintainer{
					{Email: "dev@example.com"},
				},
				Comments: []string{" maintainer-needed "},
			},
		}
		warnings := rule.Lint(".", pkg, nil)
		if len(warnings) == 0 {
			t.Error("expected warning for unneeded maintainer-needed comment, got none")
		} else if warnings[0].RuleMetadata.ID != "MaintainerNeeded" {
			t.Errorf("expected MaintainerNeeded rule, got %s", warnings[0].RuleMetadata.ID)
		}
	})

	t.Run("Has maintainer and no comment", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Maintainers: []g2.Maintainer{
					{Email: "dev@example.com"},
				},
				Comments: []string{},
			},
		}
		warnings := rule.Lint(".", pkg, nil)
		if len(warnings) > 0 {
			t.Errorf("expected no warnings, got %v", warnings)
		}
	})
}
