package ebuild_test

import (
	"strings"
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints/ebuild"
)

func TestBannedVariablesLintRule(t *testing.T) {
	rule := &ebuild.BannedVariablesLintRule{}

	tests := []struct {
		name         string
		eapi         string
		script       string
		expectedErrs int
	}{
		{
			name:   "EAPI 6 valid references",
			eapi:   "6",
			script: "PORTDIR=\"test\"\necho ${PORTDIR}\necho $ECLASSDIR\nDESTTREE=\"foo\"\nINSDESTTREE=\"bar\"\necho $DESTTREE",
			expectedErrs: 0,
		},
		{
			name:   "EAPI 7 banned references",
			eapi:   "7",
			script: "PORTDIR=\"test\"\necho ${PORTDIR}\necho $ECLASSDIR\nDESTTREE=\"foo\"\nINSDESTTREE=\"bar\"\necho $DESTTREE",
			expectedErrs: 6,
		},
		{
			name:   "EAPI 7 regular vars OK",
			eapi:   "7",
			script: "MYVAR=\"test\"\necho $MYVAR",
			expectedErrs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: "sys-apps",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"EAPI": tt.eapi,
							},
							RawText: tt.script,
						},
					},
				},
			}
			results := rule.Lint(".", pkg, nil)

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
