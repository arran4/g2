package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleSandboxFunctions = lints.RuleMetadata{
	ID:          "SandboxFunctions",
	Title:       "Sandbox Functions Usage",
	Description: "Warns about inappropriate usage of sandbox functions like addwrite, or using multiple arguments for addread, addwrite, adddeny, and addpredict.",
	URL:         "https://devmanual.gentoo.org/function-reference/sandbox-functions/index.html",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy", "sandbox"},
}

func init() {
	lints.RegisterRuleMetadata(ruleSandboxFunctions)
	lints.RegisterLintRule(&SandboxFunctionsLintRule{})
}

type SandboxFunctionsLintRule struct{}

func (l *SandboxFunctionsLintRule) Lint(repoDir string, pkgData *g2.PackageData, ctx *lints.LintContext) []lints.LintResult {
	var results []lints.LintResult

	for _, version := range pkgData.Versions {
		if version.Ebuild == nil {
			continue
		}

		// Parse the ebuild using sh syntax parser
		parser := syntax.NewParser()
		f, err := parser.Parse(strings.NewReader(version.Ebuild.RawText), "")
		if err != nil {
			continue
		}

		syntax.Walk(f, func(node syntax.Node) bool {
			cmd, ok := node.(*syntax.CallExpr)
			if !ok {
				return true
			}

			if len(cmd.Args) > 0 {
				var cmdName string
				if len(cmd.Args[0].Parts) == 1 {
					if lit, ok := cmd.Args[0].Parts[0].(*syntax.Lit); ok {
						cmdName = lit.Value
					}
				}

				if cmdName == "addwrite" {
					res := lints.LintResult{
						RuleMetadata: ruleSandboxFunctions,
						Message:      fmt.Sprintf("[Error] Ebuild %s uses addwrite, which is not an appropriate alternative to making the package build sandbox-friendly. Use addpredict instead.", version.Ebuild.Path),
						Package:      pkgData.Category + "/" + pkgData.Name,
					}
					results = append(results, res)
				}

				if cmdName == "addread" || cmdName == "addwrite" || cmdName == "adddeny" || cmdName == "addpredict" {
					if len(cmd.Args) > 2 {
						res := lints.LintResult{
							RuleMetadata: ruleSandboxFunctions,
							Message:      fmt.Sprintf("[Warning] Ebuild %s calls %s with multiple arguments. Sandbox functions do not accept multiple arguments in one call.", version.Ebuild.Path, cmdName),
							Package:      pkgData.Category + "/" + pkgData.Name,
						}
						// Override severity for this specific case as per user request
						res.RuleMetadata.Severity = lints.SeverityWarning
						results = append(results, res)
					}
				}
			}
			return true
		})
	}

	return results
}
