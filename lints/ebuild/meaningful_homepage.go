package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleMeaningfulHomepage = lints.RuleMetadata{
	ID:          "MeaningfulHomepage",
	Title:       "HOMEPAGE value must be meaningful",
	Description: "HOMEPAGE must be meaningful. Packages must not use https://www.gentoo.org/ or a similar generic homepage.",
	References: []lints.RuleReference{
		{URL: "https://projects.gentoo.org/qa/policy-guide/other-metadata.html#pg0702", Label: "Gentoo QA Policy Guide PG0702"},
	},
	Severity: lints.SeverityError,
	Source:   lints.SourceQA,
	Tags:     []string{"ebuild", "gentoo-policy", "PG0702"},
}

func init() {
	lints.RegisterRuleMetadata(ruleMeaningfulHomepage)
	lints.RegisterLintRule(&MeaningfulHomepageLintRule{})
}

type MeaningfulHomepageLintRule struct{}

func (r *MeaningfulHomepageLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *MeaningfulHomepageLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := ruleMeaningfulHomepage.Severity
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0702"]; ok {
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

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			homepage := strings.TrimSpace(ver.Ebuild.Vars["HOMEPAGE"])
			fields := strings.Fields(homepage)
			for _, hp := range fields {
				if hp == "https://www.gentoo.org/" || hp == "http://www.gentoo.org/" || hp == "https://www.gentoo.org" || hp == "http://www.gentoo.org" {
					res := lints.LintResult{
						RuleMetadata: ruleMeaningfulHomepage,
						Message:      fmt.Sprintf("[%s] Ebuild %s uses a generic Gentoo HOMEPAGE which is not meaningful", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
						Package:      pkg.Category + "/" + pkg.Name,
					}
					res.RuleMetadata.Severity = severity
					results = append(results, res)
				}
			}
		}
	}

	return results
}
