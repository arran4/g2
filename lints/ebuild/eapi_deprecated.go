package ebuild

import (
	"fmt"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func init() {
	lints.RegisterLintRule(&EAPIDeprecatedLintRule{})
}

type EAPIDeprecatedLintRule struct{}

func (r *EAPIDeprecatedLintRule) Lint(repoDir string, pkg *g2.PackageData) []string {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *EAPIDeprecatedLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []string {
	var warnings []string
	severity := "Warning"
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0803"]; ok {
			if val == "notice" || val == "error" || val == "warning" {
				severity = cases.Title(language.English).String(val)
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
				warnings = append(warnings, fmt.Sprintf("[%s] Ebuild %s uses an outdated EAPI '%s'. Consider upgrading to a newer EAPI.", severity, ver.Version, eapi))
			}
		}
	}

	return warnings
}
