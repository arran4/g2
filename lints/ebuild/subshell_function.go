package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleSubshellFunction = lints.RuleMetadata{
	ID:          "SubshellFunction",
	Title:       "Subshell Function Usage",
	Description: "Warns about the use of subshell bodies `func() ( ... )` which is not standard PMS compliant but often found in older ebuilds.",
	URL:         "https://devmanual.gentoo.org/ebuild-writing/functions/",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy"},
}

func init() {
	lints.RegisterLintRule(&SubshellFunctionLintRule{})
}

type SubshellFunctionLintRule struct{}

func (l *SubshellFunctionLintRule) Lint(repoDir string, pkgData *g2.PackageData) []lints.LintResult {
	var results []lints.LintResult

	for _, version := range pkgData.Versions {
		if version.Ebuild == nil {
			continue
		}

		for _, warning := range version.Ebuild.ParseWarnings {
			if strings.Contains(warning, "treating as subshell body") {
				res := lints.LintResult{
					RuleMetadata: ruleSubshellFunction,
					Message:      fmt.Sprintf("[Warning] Ebuild %s uses a subshell for a function body instead of braces", version.Ebuild.Path),
					Package:      pkgData.Category + "/" + pkgData.Name,
				}
				results = append(results, res)
			}
		}
	}

	return results
}
