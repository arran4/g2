package ebuild

import (
	"reflect"
	"testing"

	"github.com/arran4/g2"
)

func TestSeparateUsrLintRule(t *testing.T) {
	rule := &SeparateUsrLintRule{}

	tests := []struct {
		name         string
		pkg          *g2.PackageData
		qa           *g2.QAPolicy
		expectedMsgs []string
	}{
		{
			name: "PG0202 violation",
			pkg: &g2.PackageData{
				Category: "dev-libs",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							RawText: "dodir /bin",
						},
					},
				},
			},
			qa: &g2.QAPolicy{
				Policies: map[string]string{
					"PG0202": "error",
				},
			},
			expectedMsgs: []string{
				"[Error] Ebuild 1.0 attempts to install into /bin using dodir, which may violate separate /usr policy (PG0202).",
			},
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
