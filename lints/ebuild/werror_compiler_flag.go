package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleWerrorCompilerFlag = lints.RuleMetadata{
	ID:          "WerrorCompilerFlag",
	Title:       "-Werror compiler flag not removed",
	Description: "\"-Werror\" is a flag which turns all warnings into errors and thus will abort compiling if any warning is encountered. It is not recommended for releases and should always be disabled.",
	URL:         "https://devmanual.gentoo.org/ebuild-writing/common-mistakes/index.html#-werror-compiler-flag-not-removed",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "qa"},
}

func init() {
	lints.RegisterRuleMetadata(ruleWerrorCompilerFlag)
	lints.RegisterLintRule(&WerrorCompilerFlagLintRule{})
}

type WerrorCompilerFlagLintRule struct{}

func (l *WerrorCompilerFlagLintRule) Lint(repoDir string, pkgData *g2.PackageData, ctx *lints.LintContext) []lints.LintResult {
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
						if lit.Value == "append-flags" || lit.Value == "append-cflags" || lit.Value == "append-cxxflags" || lit.Value == "append-ldflags" {
							for _, arg := range call.Args[1:] {
								for _, part := range arg.Parts {
									if lit, ok := part.(*syntax.Lit); ok {
										if lit.Value == "-Werror" {
											res := lints.LintResult{
												RuleMetadata: ruleWerrorCompilerFlag,
												Message:      fmt.Sprintf("Ebuild %s appends -Werror compiler flag. It should be removed.", version.Ebuild.Path),
												Package:      pkgData.Category + "/" + pkgData.Name,
											}
											results = append(results, res)
										}
									}
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
