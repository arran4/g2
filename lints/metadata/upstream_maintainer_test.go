package metadata_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/metadata"
)

func TestUpstreamMaintainerLintRule(t *testing.T) {
	rule := &metadata.UpstreamMaintainerLintRule{}

	t.Run("Valid upstream maintainer", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Upstream: &g2.Upstream{
					Maintainers: []g2.Maintainer{
						{Name: "Upstream Dev"},
					},
				},
			},
		}
		warnings := rule.Lint(".", pkg)
		if len(warnings) > 0 {
			t.Errorf("expected no warnings, got %v", warnings)
		}
	})

	t.Run("Upstream maintainer with description", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Upstream: &g2.Upstream{
					Maintainers: []g2.Maintainer{
						{Name: "Upstream Dev", Description: "Primary Dev"},
					},
				},
			},
		}
		warnings := rule.Lint(".", pkg)
		if len(warnings) == 0 {
			t.Error("expected warning for description, got none")
		} else if warnings[0].RuleMetadata.ID != "UpstreamMaintainer" {
			t.Errorf("expected UpstreamMaintainer rule, got %s", warnings[0].RuleMetadata.ID)
		}
	})

	t.Run("Upstream maintainer with restrict", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Upstream: &g2.Upstream{
					Maintainers: []g2.Maintainer{
						{Name: "Upstream Dev", Restrict: "test"},
					},
				},
			},
		}
		warnings := rule.Lint(".", pkg)
		if len(warnings) == 0 {
			t.Error("expected warning for restrict, got none")
		} else if warnings[0].RuleMetadata.ID != "UpstreamMaintainer" {
			t.Errorf("expected UpstreamMaintainer rule, got %s", warnings[0].RuleMetadata.ID)
		}
	})
}
