package metadata_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/metadata"
)

func TestMaintainerTypeLintRule(t *testing.T) {
	rule := &metadata.MaintainerTypeLintRule{}

	t.Run("Valid person maintainer", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Maintainers: []g2.Maintainer{
					{Email: "person@example.com", Type: "person"},
				},
			},
		}
		warnings := rule.Lint(".", pkg)
		if len(warnings) > 0 {
			t.Errorf("expected no warnings, got %v", warnings)
		}
	})

	t.Run("Valid project maintainer", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Maintainers: []g2.Maintainer{
					{Email: "project@example.com", Type: "project"},
				},
			},
		}
		warnings := rule.Lint(".", pkg)
		if len(warnings) > 0 {
			t.Errorf("expected no warnings, got %v", warnings)
		}
	})

	t.Run("Missing type attribute", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Maintainers: []g2.Maintainer{
					{Email: "missing@example.com"},
				},
			},
		}
		warnings := rule.Lint(".", pkg)
		if len(warnings) == 0 {
			t.Error("expected warning for missing type, got none")
		} else if warnings[0].RuleMetadata.ID != "MaintainerType" {
			t.Errorf("expected MaintainerType rule, got %s", warnings[0].RuleMetadata.ID)
		}
	})

	t.Run("Invalid type attribute", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Maintainers: []g2.Maintainer{
					{Email: "unknown@example.com", Type: "unknown"},
				},
			},
		}
		warnings := rule.Lint(".", pkg)
		if len(warnings) == 0 {
			t.Error("expected warning for invalid type, got none")
		} else if warnings[0].RuleMetadata.ID != "MaintainerType" {
			t.Errorf("expected MaintainerType rule, got %s", warnings[0].RuleMetadata.ID)
		}
	})
}
