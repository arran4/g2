package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleRedefinedVariables = lints.RuleMetadata{
	ID:          "RedefinedVariables",
	Title:       "Redefined Variables",
	Description: "Checks for ebuilds that redefine variables like P, PV, PN, or PF.",
	URLs:        []string{"https://devmanual.gentoo.org/ebuild-writing/common-mistakes/index.html#redefined-p-pv-pn-pf-variables"},
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "qa"},
}

func init() {
	lints.RegisterRuleMetadata(ruleRedefinedVariables)
	lints.RegisterLintRule(&RedefinedVariablesLintRule{})
}

type RedefinedVariablesLintRule struct{}

func (l *RedefinedVariablesLintRule) Lint(repoDir string, pkgData *g2.PackageData) []lints.LintResult {
	var results []lints.LintResult

	for _, version := range pkgData.Versions {
		if version.Ebuild == nil {
			continue
		}

		parser := syntax.NewParser()
		f, err := parser.Parse(strings.NewReader(version.Ebuild.RawText), "")
		if err != nil {
			continue
		}

		syntax.Walk(f, func(node syntax.Node) bool {
			if assign, ok := node.(*syntax.Assign); ok {
				if assign.Name != nil {
					name := assign.Name.Value
					if name == "P" || name == "PV" || name == "PN" || name == "PF" || name == "PR" {
						res := lints.LintResult{
							RuleMetadata: ruleRedefinedVariables,
							Message:      fmt.Sprintf("Ebuild %s redefines %s. You should use MY_%s instead.", version.Ebuild.Path, name, name),
							Package:      pkgData.Category + "/" + pkgData.Name,
						}
						results = append(results, res)
					}
				}
			}
			return true
		})
	}

	return results
}
