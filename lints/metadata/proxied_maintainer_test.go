package metadata_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/metadata"
)

func TestProxiedMaintainerLintRule(t *testing.T) {
	rule := &metadata.ProxiedMaintainerLintRule{}

	t.Run("Has proxied but no proxy", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Maintainers: []g2.Maintainer{
					{Email: "proxied@example.com", Proxied: "yes"},
				},
			},
		}
		warnings := rule.Lint(".", pkg, nil)
		if len(warnings) == 0 {
			t.Error("expected warning for missing proxy, got none")
		} else if warnings[0].RuleMetadata.ID != "ProxiedMaintainer" {
			t.Errorf("expected ProxiedMaintainer rule, got %s", warnings[0].RuleMetadata.ID)
		}
	})

	t.Run("Has proxy but no proxied", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Maintainers: []g2.Maintainer{
					{Email: "proxy@example.com", Proxied: "proxy"},
				},
			},
		}
		warnings := rule.Lint(".", pkg, nil)
		if len(warnings) == 0 {
			t.Error("expected warning for missing proxied, got none")
		} else if warnings[0].RuleMetadata.ID != "ProxiedMaintainer" {
			t.Errorf("expected ProxiedMaintainer rule, got %s", warnings[0].RuleMetadata.ID)
		}
	})

	t.Run("Has both", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Maintainers: []g2.Maintainer{
					{Email: "proxied@example.com", Proxied: "yes"},
					{Email: "proxy@example.com", Proxied: "proxy"},
				},
			},
		}
		warnings := rule.Lint(".", pkg, nil)
		if len(warnings) > 0 {
			t.Errorf("expected no warnings, got %v", warnings)
		}
	})

	t.Run("Has neither", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Maintainers: []g2.Maintainer{
					{Email: "maintainer@example.com", Proxied: "no"},
				},
			},
		}
		warnings := rule.Lint(".", pkg, nil)
		if len(warnings) > 0 {
			t.Errorf("expected no warnings, got %v", warnings)
		}
	})
}
