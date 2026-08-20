package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleUseConditionalCode = lints.RuleMetadata{
	ID:          "UseConditionalCode",
	Title:       "USE flag conditional code",
	Description: "Checks for deprecated useq and invalid [ \"`use foo`\" ] syntax.",
	URLs:        []string{"https://devmanual.gentoo.org/ebuild-writing/use-conditional-code/index.html"},
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy", "use"},
}

func init() {
	lints.RegisterRuleMetadata(ruleUseConditionalCode)
	lints.RegisterLintRule(&UseConditionalCodeLintRule{})
}

type UseConditionalCodeLintRule struct{}

func (l *UseConditionalCodeLintRule) Lint(repoDir string, pkgData *g2.PackageData) []lints.LintResult {
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
			if call, ok := node.(*syntax.CallExpr); ok && len(call.Args) > 0 {
				if len(call.Args[0].Parts) == 1 {
					if lit, ok := call.Args[0].Parts[0].(*syntax.Lit); ok {
						if lit.Value == "useq" {
							res := lints.LintResult{
								RuleMetadata: ruleUseConditionalCode,
								Message:      fmt.Sprintf("[Warning] Ebuild %s uses 'useq', which is a deprecated synonym for 'use'.", version.Version),
								Package:      pkgData.Category + "/" + pkgData.Name,
							}
							res.RuleMetadata.Severity = lints.SeverityWarning
							results = append(results, res)
						}
					}
				}
			}

			if subst, ok := node.(*syntax.CmdSubst); ok {
				// Check if the command substitution consists of exactly one statement,
				// and that statement is just a bare call to `use`.
				if len(subst.Stmts) == 1 {
					stmt := subst.Stmts[0]
					if stmt.Background || stmt.Negated || stmt.Redirs != nil {
						return true
					}
					if call, ok := stmt.Cmd.(*syntax.CallExpr); ok && len(call.Args) > 0 {
						if len(call.Args[0].Parts) == 1 {
							if lit, ok := call.Args[0].Parts[0].(*syntax.Lit); ok {
								if lit.Value == "use" {
									res := lints.LintResult{
										RuleMetadata: ruleUseConditionalCode,
										Message:      fmt.Sprintf("[Error] Ebuild %s uses 'use' inside a command substitution (e.g. \"`use foo`\"). The 'use' function does not produce output, so this will evaluate to an empty string.", version.Version),
										Package:      pkgData.Category + "/" + pkgData.Name,
									}
									results = append(results, res)
								}
							}
						}
					}
				}
			}
			return true
		})
	}

	return results
}
