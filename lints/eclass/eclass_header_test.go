package eclass

import (
	"testing"

	"github.com/arran4/g2"
)

func TestEclassHeaderLintRule(t *testing.T) {
	tests := []struct {
		name       string
		eclass     *g2.Ebuild
		qa         *g2.QAPolicy
		wantCount  int
		wantErrors []string
	}{
		{
			name: "All tags present",
			eclass: &g2.Ebuild{
				Path: "eclass/test.eclass",
				RawText: `# @ECLASS: test.eclass
# @MAINTAINER: test@example.com
# @BLURB: test blurb
`,
			},
			wantCount: 0,
		},
		{
			name: "Missing BLURB tag",
			eclass: &g2.Ebuild{
				Path: "eclass/test.eclass",
				RawText: `# @ECLASS: test.eclass
# @MAINTAINER: test@example.com
`,
			},
			wantCount: 1,
			wantErrors: []string{
				"[Warning] Eclass test.eclass is missing the required @BLURB: documentation tag.",
			},
		},
		{
			name: "Missing MAINTAINER tag",
			eclass: &g2.Ebuild{
				Path: "eclass/test.eclass",
				RawText: `# @ECLASS: test.eclass
# @BLURB: test blurb
`,
			},
			wantCount: 1,
			wantErrors: []string{
				"[Warning] Eclass test.eclass is missing the required @MAINTAINER: documentation tag.",
			},
		},
		{
			name: "Missing ECLASS tag",
			eclass: &g2.Ebuild{
				Path: "eclass/test.eclass",
				RawText: `# @MAINTAINER: test@example.com
# @BLURB: test blurb
`,
			},
			wantCount: 1,
			wantErrors: []string{
				"[Warning] Eclass test.eclass is missing the required @ECLASS: documentation tag.",
			},
		},
		{
			name: "Missing all tags",
			eclass: &g2.Ebuild{
				Path:    "eclass/test.eclass",
				RawText: ``,
			},
			wantCount: 3,
			wantErrors: []string{
				"[Warning] Eclass test.eclass is missing the required @ECLASS: documentation tag.",
				"[Warning] Eclass test.eclass is missing the required @MAINTAINER: documentation tag.",
				"[Warning] Eclass test.eclass is missing the required @BLURB: documentation tag.",
			},
		},
		{
			name:      "Nil eclass",
			eclass:    nil,
			wantCount: 0,
		},
		{
			name: "QA Policy Ignore",
			eclass: &g2.Ebuild{
				Path: "eclass/test.eclass",
				RawText: `# @ECLASS: test.eclass
# @MAINTAINER: test@example.com
`,
			},
			qa: &g2.QAPolicy{
				Policies: map[string]string{"PG0804": "ignore"},
			},
			wantCount: 0,
		},
		{
			name: "QA Policy Error",
			eclass: &g2.Ebuild{
				Path: "eclass/test.eclass",
				RawText: `# @ECLASS: test.eclass
# @MAINTAINER: test@example.com
`,
			},
			qa: &g2.QAPolicy{
				Policies: map[string]string{"PG0804": "error"},
			},
			wantCount: 1,
			wantErrors: []string{
				"[Error] Eclass test.eclass is missing the required @BLURB: documentation tag.",
			},
		},
		{
			name: "QA Policy Notice",
			eclass: &g2.Ebuild{
				Path: "eclass/test.eclass",
				RawText: `# @ECLASS: test.eclass
# @MAINTAINER: test@example.com
`,
			},
			qa: &g2.QAPolicy{
				Policies: map[string]string{"PG0804": "notice"},
			},
			wantCount: 1,
			wantErrors: []string{
				"[Notice] Eclass test.eclass is missing the required @BLURB: documentation tag.",
			},
		},
	}

	rule := &EclassHeaderLintRule{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := rule.LintWithQA(".", tt.eclass, tt.qa, nil)
			if len(results) != tt.wantCount {
				t.Errorf("expected %d results, got %d", tt.wantCount, len(results))
			}
			if tt.wantCount > 0 && len(results) == tt.wantCount {
				for i, want := range tt.wantErrors {
					if results[i].Message != want {
						t.Errorf("expected result message %q, got %q", want, results[i].Message)
					}
				}
			}
		})
	}
}
