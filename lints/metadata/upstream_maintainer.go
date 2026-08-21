package metadata

import (
	"fmt"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleUpstreamMaintainer = lints.RuleMetadata{
	ID:          "UpstreamMaintainer",
	Title:       "Upstream Maintainer Rule",
	Description: "Ensures that maintainers listed inside the <upstream> block do not use description or restrict tags/attributes.",
	References: []lints.RuleReference{
		{URL: "https://www.gentoo.org/glep/glep-0068.html#upstream-block", Label: "GLEP 68"},
	},
	Severity: lints.SeverityError,
	Source:   lints.SourceG2,
	Tags:     []string{"metadata.xml", "site-quality", "GLEP68"},
}

func init() {
	lints.RegisterRuleMetadata(ruleUpstreamMaintainer)
	lints.RegisterLintRule(&UpstreamMaintainerLintRule{})
}

type UpstreamMaintainerLintRule struct{}

func (r *UpstreamMaintainerLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *UpstreamMaintainerLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["GLEP68"]; ok {
			if val == "ignore" {
				return nil
			}
			if val == "notice" || val == "error" || val == "warning" {
				switch val {
				case "notice":
					severity = lints.SeverityNotice
				case "error":
					severity = lints.SeverityError
				case "warning":
					severity = lints.SeverityWarning
				}
			}
		}
	}

	if pkg.Metadata != nil && pkg.Metadata.Upstream != nil {
		for _, m := range pkg.Metadata.Upstream.Maintainers {
			if m.Description != "" {
				res := lints.LintResult{
					RuleMetadata: ruleUpstreamMaintainer,
					Message:      fmt.Sprintf("[%s] Upstream maintainer '%s' must not have a description", cases.Title(language.Und, cases.NoLower).String(string(severity)), m.Name),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}
			if m.Restrict != "" {
				res := lints.LintResult{
					RuleMetadata: ruleUpstreamMaintainer,
					Message:      fmt.Sprintf("[%s] Upstream maintainer '%s' must not have a restrict attribute", cases.Title(language.Und, cases.NoLower).String(string(severity)), m.Name),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}
		}
	}

	return results
}
