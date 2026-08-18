package ebuild_test

import (
	"strings"
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"github.com/arran4/g2/lints/ebuild"
)

func TestEAPIDeprecatedLintRule(t *testing.T) {
	rule := &ebuild.EAPIDeprecatedLintRule{}

	tests := []struct {
		name     string
		eapi     string
		hasErr   bool
		hasWarn  bool
	}{
		{"EAPI 0", "0", true, false},
		{"EAPI 4", "4", true, false},
		{"EAPI 5", "5", false, true},
		{"EAPI 6", "6", false, true},
		{"EAPI 7", "7", false, false},
		{"EAPI 8", "8", false, false},
		{"Missing EAPI (defaults to 0)", "", true, false},
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

			errs := 0
			warns := 0
			for _, res := range warnings {
				if res.RuleMetadata.Severity == lints.SeverityError {
					errs++
				} else if res.RuleMetadata.Severity == lints.SeverityWarning {
					warns++
				}
			}

			if tt.hasErr && errs == 0 {
				t.Errorf("expected error for EAPI %s, got none", tt.eapi)
			}
			if !tt.hasErr && errs > 0 {
				t.Errorf("expected no error for EAPI %s, got %d", tt.eapi, errs)
			}
			if tt.hasWarn && warns == 0 {
				t.Errorf("expected warning for EAPI %s, got none", tt.eapi)
			}
			if !tt.hasWarn && warns > 0 {
				t.Errorf("expected no warning for EAPI %s, got %d", tt.eapi, warns)
			}

			if len(warnings) > 0 {
				if !strings.Contains(warnings[0].Message, "EAPI") {
					t.Errorf("expected warning to mention EAPI, got: %s", warnings[0].Message)
				}
			}
		})
	}
}
