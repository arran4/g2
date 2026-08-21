package ebuild

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleDieSubshell = lints.RuleMetadata{
	ID:          "DieSubshell",
	Title:       "die in subshell",
	Description: "die will not work in a subshell unless you are using EAPI=7 and onwards.",
	References: []lints.RuleReference{
		{URL: "https://devmanual.gentoo.org/ebuild-writing/error-handling/index.html#die-and-subshells", Label: "Gentoo Devmanual"},
	},
	Severity: lints.SeverityWarning,
	Source:   lints.SourceG2,
	Tags:     []string{"ebuild", "gentoo-policy", "die"},
}

func init() {
	lints.RegisterRuleMetadata(ruleDieSubshell)
	lints.RegisterLintRule(&DieSubshellLintRule{})
}

type DieSubshellLintRule struct{}

func (l *DieSubshellLintRule) Lint(repoDir string, pkgData *g2.PackageData) []lints.LintResult {
	var results []lints.LintResult

	for _, version := range pkgData.Versions {
		if version.Ebuild == nil {
			continue
		}

		eapi := version.Ebuild.Vars["EAPI"]
		if eapi == "" {
			eapi = "0"
		}

		eapiNum, err := strconv.Atoi(eapi)
		if err != nil {
			// If EAPI is not a number, it's likely a non-standard or very new one,
			// assume it's modern enough (or fallback to strict behavior)
			// But for Gentoo, we only care if it's explicitly parsed as < 7.
			// e.g. "4-slot-abi" -> we don't handle easily, but let's assume it's EAPI 4.
			// Actually let's just do a simple prefix check for old EAPIs if it's not a pure number.
			if strings.HasPrefix(eapi, "4-") {
				eapiNum = 4
			} else {
				eapiNum = 8 // Default to safe if not purely numeric to avoid false positives
			}
		}

		// "die will not work in a subshell unless you are using EAPI=7 and onwards."
		if eapiNum >= 7 {
			continue
		}

		// Parse the ebuild using sh syntax parser
		parser := syntax.NewParser()
		f, err := parser.Parse(strings.NewReader(version.Ebuild.RawText), "")
		if err != nil {
			continue
		}

		seen := make(map[*syntax.CallExpr]bool)

		syntax.Walk(f, func(node syntax.Node) bool {
			isSubshell := false
			switch n := node.(type) {
			case *syntax.Subshell, *syntax.CmdSubst, *syntax.ProcSubst, *syntax.CoprocClause:
				isSubshell = true
			case *syntax.BinaryCmd:
				if n.Op == syntax.Pipe || n.Op == syntax.PipeAll {
					isSubshell = true
				}
			case *syntax.Stmt:
				if n.Background {
					isSubshell = true
				}
			}

			if isSubshell {
				syntax.Walk(node, func(child syntax.Node) bool {
					if call, ok := child.(*syntax.CallExpr); ok {
						if len(call.Args) > 0 && len(call.Args[0].Parts) == 1 {
							if lit, ok := call.Args[0].Parts[0].(*syntax.Lit); ok && lit.Value == "die" {
								if !seen[call] {
									res := lints.LintResult{
										RuleMetadata: ruleDieSubshell,
										Message:      fmt.Sprintf("[Warning] Ebuild %s uses 'die' in a subshell, but EAPI %s does not support this. The 'die' call will not abort the build as expected.", version.Ebuild.Path, eapi),
										Package:      pkgData.Category + "/" + pkgData.Name,
									}
									results = append(results, res)
									seen[call] = true
								}
							}
						}
					}
					return true
				})
				return false // Do not process this subshell further from the outer loop
			}
			return true
		})
	}

	return results
}
