package metadata_test

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/metadata"
)

func TestRemoteIDTypeLintRule(t *testing.T) {
	rule := &metadata.RemoteIDTypeLintRule{}

	t.Run("Valid remote id", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Upstream: &g2.Upstream{
					RemoteID: []g2.RemoteID{
						{Type: "github", Text: "foo/bar"},
						{Type: "pypi", Text: "foo"},
					},
				},
			},
		}
		warnings := rule.Lint(".", pkg)
		if len(warnings) > 0 {
			t.Errorf("expected no warnings, got %v", warnings)
		}
	})

	t.Run("Invalid remote id", func(t *testing.T) {
		pkg := &g2.PackageData{
			Metadata: &g2.PkgMetadata{
				Upstream: &g2.Upstream{
					RemoteID: []g2.RemoteID{
						{Type: "invalid-type", Text: "foo"},
					},
				},
			},
		}
		warnings := rule.Lint(".", pkg)
		if len(warnings) == 0 {
			t.Error("expected warning for invalid remote-id type, got none")
		} else if warnings[0].RuleMetadata.ID != "RemoteIDType" {
			t.Errorf("expected RemoteIDType rule, got %s", warnings[0].RuleMetadata.ID)
		}
	})
}
