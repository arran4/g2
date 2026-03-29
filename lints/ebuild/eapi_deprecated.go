package ebuild

import (
	"fmt"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleEAPIDeprecated = lints.RuleMetadata{
	ID:          "EAPIDeprecated",
	Title:       "Deprecated EAPI",
	Description: "Detects the use of old, deprecated EAPIs.",
	URL:         "https://devmanual.gentoo.org/ebuild-writing/eapi/",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy"},
}

func init() {
	lints.RegisterLintRule(&EAPIDeprecatedLintRule{})
}

type EAPIDeprecatedLintRule struct{}

func (r *EAPIDeprecatedLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *EAPIDeprecatedLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityWarning
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0803"]; ok {
			if val == "notice" || val == "error" || val == "warning" {
				// Convert val to lints.Severity (simple mapping)
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
			eapi := ver.Ebuild.Vars["EAPI"]
			if eapi == "" {
				eapi = "0"
			}

			// Simple check for EAPI < 6
			isDeprecated := false
			if len(eapi) == 1 && eapi[0] >= '0' && eapi[0] <= '5' {
				isDeprecated = true
			}

			if isDeprecated {
				res := lints.LintResult{
					RuleMetadata: ruleEAPIDeprecated,
					Message:      fmt.Sprintf("[%s] Ebuild %s uses an outdated EAPI '%s'. Consider upgrading to a newer EAPI.", cases.Title(language.English).String(string(severity)), ver.Version, eapi),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}
		}
	}

	return results
}
