package ebuild

import (
	"fmt"
	"strconv"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleEAPIDeprecated = lints.RuleMetadata{
	ID:          "EAPIDeprecated",
	Title:       "Deprecated EAPI",
	Description: "Detects the use of old, deprecated or obsolete EAPIs.",
	URL:         "https://devmanual.gentoo.org/ebuild-writing/eapi/",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy", "PG0803"},
}

func init() {
	lints.RegisterRuleMetadata(ruleEAPIDeprecated)
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
			if val == "ignore" {
				return nil
			}
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

			eapiNum, err := strconv.Atoi(eapi)
			if err != nil {
				continue // Skip if not a valid number
			}

			if eapiNum >= 0 && eapiNum <= 4 {
				res := lints.LintResult{
					RuleMetadata: ruleEAPIDeprecated,
					Message:      fmt.Sprintf("[Error] Ebuild %s uses an obsolete EAPI '%s'. EAPIs 0 to 4 are obsolete and must no longer be used.", ver.Version, eapi),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				// Force error severity for obsolete EAPIs regardless of QA policy, as they are strictly banned
				res.RuleMetadata.Severity = lints.SeverityError
				results = append(results, res)
			} else if eapiNum == 5 || eapiNum == 6 {
				res := lints.LintResult{
					RuleMetadata: ruleEAPIDeprecated,
					Message:      fmt.Sprintf("[%s] Ebuild %s uses a deprecated EAPI '%s'. Consider upgrading to a newer EAPI.", severity, ver.Version, eapi),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}
		}
	}

	return results
}
