package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleLicense = lints.RuleMetadata{
	ID:          "License",
	Title:       "LICENSE variable must explicitly list all licenses",
	Description: "The LICENSE variable must explicitly list all licenses pertaining to the corresponding source of the files installed by the package.",
	URL:         "https://projects.gentoo.org/qa/policy-guide/other-metadata.html#pg0704",
	Severity:    lints.SeverityError,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "PG0704"},
}

func init() {
	lints.RegisterRuleMetadata(ruleLicense)
	lints.RegisterLintRule(&LicenseLintRule{})
}

type LicenseLintRule struct{}

func (r *LicenseLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *LicenseLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := ruleLicense.Severity
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0704"]; ok {
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
			license := strings.TrimSpace(ver.Ebuild.Vars["LICENSE"])

			// Virtuals don't require LICENSE
			if pkg.Category == "virtual" {
			    continue
			}

			if license == "" {
				res := lints.LintResult{
					RuleMetadata: ruleLicense,
					Message:      fmt.Sprintf("[%s] Ebuild %s does not specify LICENSE", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}
		}
	}

	return results
}
