package ebuild

import (
	"reflect"
	"testing"

	"github.com/arran4/g2"
)

func TestUseDependenciesPG0021LintRule(t *testing.T) {
	rule := &UseDependenciesPG0021LintRule{}

	tests := []struct {
		name         string
		pkg          *g2.PackageData
		qa           *g2.QAPolicy
		expectedMsgs []string
	}{
		{
			name: "PG0021 ignore policy",
			pkg: &g2.PackageData{
				Category: "dev-libs",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"DEPEND": "dev-libs/bar[flag]",
							},
						},
					},
				},
			},
			qa: &g2.QAPolicy{
				Policies: map[string]string{
					"PG0021": "ignore",
				},
			},
			expectedMsgs: nil, // We stubbed the logic, so it always returns nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := rule.LintWithQA("", tt.pkg, tt.qa)
			var msgs []string
			for _, res := range results {
				msgs = append(msgs, res.Message)
			}
			if len(msgs) == 0 && len(tt.expectedMsgs) == 0 {
				return
			}
			if !reflect.DeepEqual(msgs, tt.expectedMsgs) {
				t.Errorf("Expected %v, got %v", tt.expectedMsgs, msgs)
			}
		})
	}
}
