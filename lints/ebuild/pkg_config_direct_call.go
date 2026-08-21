package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var rulePkgConfigDirectCall = lints.RuleMetadata{
	ID:          "PkgConfigDirectCall",
	Title:       "Calling pkg-config directly",
	Description: "You should not call pkg-config directly in ebuilds because this is problematic for e.g. cross-compiling. Instead, use tc-getPKG_CONFIG from toolchain-funcs.eclass.",
	References: []lints.RuleReference{
		{URL: "https://devmanual.gentoo.org/ebuild-writing/common-mistakes/index.html#calling-pkg-config-directly", Label: "Gentoo Devmanual"},
	},
	Severity: lints.SeverityError,
	Source:   lints.SourceG2,
	Tags:     []string{"ebuild", "qa"},
}

func init() {
	lints.RegisterRuleMetadata(rulePkgConfigDirectCall)
	lints.RegisterLintRule(&PkgConfigDirectCallLintRule{})
}

type PkgConfigDirectCallLintRule struct{}

func (l *PkgConfigDirectCallLintRule) Lint(repoDir string, pkgData *g2.PackageData) []lints.LintResult {
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
				if len(call.Args[0].Parts) > 0 {
					if lit, ok := call.Args[0].Parts[0].(*syntax.Lit); ok {
						if lit.Value == "pkg-config" {
							res := lints.LintResult{
								RuleMetadata: rulePkgConfigDirectCall,
								Message:      fmt.Sprintf("Ebuild %s calls pkg-config directly. You should use $(tc-getPKG_CONFIG) instead.", version.Ebuild.Path),
								Package:      pkgData.Category + "/" + pkgData.Name,
							}
							results = append(results, res)
						}
					}
				}
			}
			return true
		})
	}

	return results
}
