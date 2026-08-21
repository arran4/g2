package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"mvdan.cc/sh/v3/syntax"
)

var ruleCodingStyle = lints.RuleMetadata{
	ID:          "CodingStyle",
	Title:       "Coding Style",
	Description: "Validates ebuild coding style: no POSIX test ([ ... ] or test), use bracketed variables (${foo}).",
	References: []lints.RuleReference{
		{URL: "https://projects.gentoo.org/qa/policy-guide/ebuild-format.html#pg0101", Label: "Gentoo QA Policy Guide PG0101"},
	},
	Severity: lints.SeverityWarning,
	Source:   lints.SourceQA,
	Tags:     []string{"ebuild", "gentoo-policy", "PG0101"},
}

func init() {
	lints.RegisterRuleMetadata(ruleCodingStyle)
	lints.RegisterLintRule(&CodingStyleLintRule{})
}

type CodingStyleLintRule struct{}

func (r *CodingStyleLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *CodingStyleLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityWarning

	if qa != nil {
		if val, ok := qa.Policies["PG0101"]; ok { // Coding style
			if val == "ignore" {
				return nil
			}
			switch val {
			case "error":
				severity = lints.SeverityError
			case "warning":
				severity = lints.SeverityWarning
			case "notice":
				severity = lints.SeverityNotice
			}
		}
	}

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.RawText != "" {
			parser := syntax.NewParser(syntax.KeepComments(true))
			f, err := parser.Parse(strings.NewReader(ver.Ebuild.RawText), "")
			if err != nil {
				continue // handled by syntax error lint
			}

			syntax.Walk(f, func(node syntax.Node) bool {
				switch n := node.(type) {
				case *syntax.ParamExp:
					if n.Short {
						// Exclude special bash variables
						if len(n.Param.Value) == 1 && strings.ContainsAny(n.Param.Value, "0123456789@*?$-#!") {
							// skip
						} else {
							res := lints.LintResult{
								RuleMetadata: ruleCodingStyle,
								Message:      fmt.Sprintf("[%s] Ebuild %s uses unbracketed variable '$%s'. Use '${%s}' instead.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version, n.Param.Value, n.Param.Value),
								Package:      pkg.Category + "/" + pkg.Name,
							}
							res.RuleMetadata.Severity = severity
							results = append(results, res)
						}
					}
				case *syntax.CallExpr:
					if len(n.Args) > 0 && len(n.Args[0].Parts) > 0 {
						if lit, ok := n.Args[0].Parts[0].(*syntax.Lit); ok {
							cmd := lit.Value
							if cmd == "[" || cmd == "test" {
								res := lints.LintResult{
									RuleMetadata: ruleCodingStyle,
									Message:      fmt.Sprintf("[%s] Ebuild %s uses POSIX test '%s'. Use bash conditions '[[ ... ]]' instead.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version, cmd),
									Package:      pkg.Category + "/" + pkg.Name,
								}
								res.RuleMetadata.Severity = severity
								results = append(results, res)
							}
						}
					}
				}
				return true
			})
		}
	}
	return results
}
