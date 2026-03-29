package ebuild_test

import (
	"strings"
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestEAPIDeprecatedLintRule(t *testing.T) {
	rule := &ebuild.EAPIDeprecatedLintRule{}

	tests := []struct {
		name     string
		eapi     string
		hasWarn  bool
	}{
		{"EAPI 5", "5", true},
		{"EAPI 0", "0", true},
		{"EAPI 6", "6", false},
		{"EAPI 7", "7", false},
		{"EAPI 8", "8", false},
		{"Missing EAPI (defaults to 0)", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"EAPI": tt.eapi,
							},
						},
					},
				},
			}
			warnings := rule.Lint(".", pkg)
			if tt.hasWarn && len(warnings) == 0 {
				t.Errorf("expected warning for EAPI %s, got none", tt.eapi)
			}
			if !tt.hasWarn && len(warnings) > 0 {
				t.Errorf("expected no warning for EAPI %s, got %v", tt.eapi, warnings)
			}
			if tt.hasWarn && len(warnings) > 0 {
				if !strings.Contains(warnings[0].Message, "EAPI") {
					t.Errorf("expected warning to mention EAPI, got: %s", warnings[0].Message)
				}
			}
		})
	}
}
