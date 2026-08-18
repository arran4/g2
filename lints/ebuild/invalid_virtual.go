package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleInvalidVirtual = lints.RuleMetadata{
	ID:          "InvalidVirtual",
	Title:       "Invalid Virtual",
	Description: "Checks if a virtual package defines HOMEPAGE or LICENSE variables, which it should not.",
	URL:         "https://devmanual.gentoo.org/general-concepts/virtuals/index.html",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy"},
}

func init() {
	lints.RegisterRuleMetadata(ruleInvalidVirtual)
	lints.RegisterLintRule(&InvalidVirtualLintRule{})
}

type InvalidVirtualLintRule struct{}

func (r *InvalidVirtualLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *InvalidVirtualLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError

	if pkg.Category != "virtual" {
		return nil
	}

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			homepage := strings.TrimSpace(ver.Ebuild.Vars["HOMEPAGE"])
			license := strings.TrimSpace(ver.Ebuild.Vars["LICENSE"])
			srcUri := strings.TrimSpace(ver.Ebuild.Vars["SRC_URI"])

			if homepage != "" {
				res := lints.LintResult{
					RuleMetadata: ruleInvalidVirtual,
					Message:      fmt.Sprintf("[%s] Ebuild %s is a virtual but defines HOMEPAGE", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}

			if license != "" {
				res := lints.LintResult{
					RuleMetadata: ruleInvalidVirtual,
					Message:      fmt.Sprintf("[%s] Ebuild %s is a virtual but defines LICENSE", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}

			if srcUri != "" {
				res := lints.LintResult{
					RuleMetadata: ruleInvalidVirtual,
					Message:      fmt.Sprintf("[%s] Ebuild %s is a virtual but defines SRC_URI", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}
		}
	}
	return results
}
