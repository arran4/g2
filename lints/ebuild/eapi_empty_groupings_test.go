package ebuild_test

import (
	"strings"
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestEAPIEmptyGroupingsLintRule(t *testing.T) {
	rule := &ebuild.EAPIEmptyGroupingsLintRule{}

	tests := []struct {
		name         string
		eapi         string
		vars         map[string]string
		expectedErrs int
	}{
		{
			name: "EAPI 6 empty group",
			eapi: "6",
			vars: map[string]string{
				"DEPEND": "|| ( )",
			},
			expectedErrs: 0,
		},
		{
			name: "EAPI 7 empty group in DEPEND",
			eapi: "7",
			vars: map[string]string{
				"DEPEND": "|| ( )",
			},
			expectedErrs: 1,
		},
		{
			name: "EAPI 7 empty group with spaces",
			eapi: "7",
			vars: map[string]string{
				"RDEPEND": "|| (   )",
			},
			expectedErrs: 1,
		},
		{
			name: "EAPI 7 empty group with text",
			eapi: "7",
			vars: map[string]string{
				"DEPEND": "|| ( foo )",
			},
			expectedErrs: 0,
		},
		{
			name: "EAPI 7 empty group multiple",
			eapi: "7",
			vars: map[string]string{
				"DEPEND": "|| ( ) foo || ( )",
			},
			expectedErrs: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := tt.vars
			if vars == nil {
				vars = make(map[string]string)
			}
			vars["EAPI"] = tt.eapi

			pkg := &g2.PackageData{
				Category: "sys-apps",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: vars,
						},
					},
				},
			}
			results := rule.Lint(".", pkg)

			errs := 0
			for _, res := range results {
				if strings.HasPrefix(res.Message, "[Error]") {
					errs++
				}
			}

			if errs != tt.expectedErrs {
				t.Errorf("expected %d errors, got %d. Results: %v", tt.expectedErrs, errs, results)
			}
		})
	}
}
