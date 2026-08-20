package ebuild

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/stretchr/testify/assert"
)

func TestGLEP81UserGroupLintRule(t *testing.T) {
	rule := &GLEP81UserGroupLintRule{}

	tests := []struct {
		name          string
		rawText       string
		vars          map[string]string
		qa            *g2.QAPolicy
		expectedError int
	}{
		{
			name: "Valid no user eclass",
			rawText: `
				src_install() {
					emake install
				}
			`,
			vars:          map[string]string{"INHERITED": "eutils multilib"},
			expectedError: 0,
		},
		{
			name: "Inherits user eclass",
			rawText: `
				src_install() {
					emake install
				}
			`,
			vars:          map[string]string{"INHERITED": "user multilib"},
			expectedError: 1,
		},
		{
			name: "Uses enewuser",
			rawText: `
				pkg_setup() {
					enewuser myuser
				}
			`,
			vars:          map[string]string{},
			expectedError: 1,
		},
		{
			name: "Uses enewgroup",
			rawText: `
				pkg_setup() {
					enewgroup mygroup
				}
			`,
			vars:          map[string]string{},
			expectedError: 1,
		},
		{
			name: "Uses both user eclass and enewuser",
			rawText: `
				pkg_setup() {
					enewuser myuser
				}
			`,
			vars:          map[string]string{"INHERITED": "user multilib"},
			expectedError: 2, // One for eclass, one for function call
		},
		{
			name: "Ignore via QA policy",
			rawText: `
				pkg_setup() {
					enewuser myuser
				}
			`,
			vars: map[string]string{"INHERITED": "user"},
			qa: &g2.QAPolicy{
				Policies: map[string]string{"PG0901": "ignore"},
			},
			expectedError: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: "app-test",
				Name:     "testpkg",
				Versions: []g2.VersionData{
					{
						Ebuild: &g2.Ebuild{
							RawText: tt.rawText,
							Vars:    tt.vars,
							Path:    "app-test/testpkg/testpkg-1.ebuild",
						},
					},
				},
			}

			results := rule.LintWithQA(".", pkg, tt.qa)
			assert.Equal(t, tt.expectedError, len(results))

			if tt.expectedError > 0 {
				for _, res := range results {
					assert.Equal(t, ruleGLEP81UserGroup.ID, res.RuleMetadata.ID)
					// assert.Contains(t, res.Message, "GLEP 81")
				}
			}
		})
	}
}
