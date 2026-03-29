package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestEAPIDeprecatedLintRule(t *testing.T) {
	rule := &EAPIDeprecatedLintRule{}

	t.Run("Modern EAPI", func(t *testing.T) {
		pkg := &g2.PackageData{
			Versions: []g2.VersionData{
				{
					Version: "1.0",
					Ebuild: &g2.Ebuild{
						Vars: map[string]string{"EAPI": "8"},
					},
				},
			},
		}
		warnings := rule.Lint("", pkg)
		if len(warnings) != 0 {
			t.Errorf("Expected 0 warnings, got %v", warnings)
		}
	})

	t.Run("Deprecated EAPI", func(t *testing.T) {
		pkg := &g2.PackageData{
			Versions: []g2.VersionData{
				{
					Version: "1.0",
					Ebuild: &g2.Ebuild{
						Vars: map[string]string{"EAPI": "5"},
					},
				},
			},
		}
		warnings := rule.Lint("", pkg)
		if len(warnings) != 1 {
			t.Errorf("Expected 1 warning, got %v", warnings)
		} else if warnings[0] != "[Warning] Ebuild 1.0 uses an outdated EAPI '5'. Consider upgrading to a newer EAPI." {
			t.Errorf("Unexpected warning: %s", warnings[0])
		}
	})

	t.Run("Missing EAPI", func(t *testing.T) {
		pkg := &g2.PackageData{
			Versions: []g2.VersionData{
				{
					Version: "1.0",
					Ebuild: &g2.Ebuild{
						Vars: map[string]string{}, // Defaults to 0
					},
				},
			},
		}
		warnings := rule.Lint("", pkg)
		if len(warnings) != 1 {
			t.Errorf("Expected 1 warning, got %v", warnings)
		} else if warnings[0] != "[Warning] Ebuild 1.0 uses an outdated EAPI '0'. Consider upgrading to a newer EAPI." {
			t.Errorf("Unexpected warning: %s", warnings[0])
		}
	})
}
