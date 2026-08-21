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

var ruleDVariables = lints.RuleMetadata{
	ID:          "DVariables",
	Title:       "D used outside src_install and pkg_preinst",
	Description: "The D and ED variables must be used only in the src_install and pkg_preinst phase functions.",
	References: []lints.RuleReference{
		{URL: "https://projects.gentoo.org/qa/policy-guide/ebuild-format.html#pg0107", Label: "Gentoo QA Policy Guide PG0107"},
	},
	Severity: lints.SeverityError,
	Source:   lints.SourceQA,
	Tags:     []string{"ebuild", "gentoo-policy", "PG0107"},
}

func init() {
	lints.RegisterRuleMetadata(ruleDVariables)
	lints.RegisterLintRule(&DVariablesLintRule{})
}

type DVariablesLintRule struct{}

func (r *DVariablesLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *DVariablesLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError

	if qa != nil {
		if val, ok := qa.Policies["PG0107"]; ok {
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
				continue
			}

			syntax.Walk(f, func(node syntax.Node) bool {
				switch n := node.(type) {
				case *syntax.FuncDecl:
					funcName := n.Name.Value
					if funcName != "src_install" && funcName != "pkg_preinst" {
						syntax.Walk(n, func(inner syntax.Node) bool {
							switch nx := inner.(type) {
							case *syntax.ParamExp:
								if nx.Param.Value == "D" || nx.Param.Value == "ED" {
									res := lints.LintResult{
										RuleMetadata: ruleDVariables,
										Message:      fmt.Sprintf("[%s] Ebuild %s uses '${%s}' in '%s'. It must be used only in src_install and pkg_preinst.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version, nx.Param.Value, funcName),
										Package:      pkg.Category + "/" + pkg.Name,
									}
									res.RuleMetadata.Severity = severity
									results = append(results, res)
								}
							}
							return true
						})
					}
					return false // do not walk inside the function again from global walk
				case *syntax.ParamExp: // Global scope check
					if n.Param.Value == "D" || n.Param.Value == "ED" {
						res := lints.LintResult{
							RuleMetadata: ruleDVariables,
							Message:      fmt.Sprintf("[%s] Ebuild %s uses '${%s}' in global scope. It must be used only in src_install and pkg_preinst.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version, n.Param.Value),
							Package:      pkg.Category + "/" + pkg.Name,
						}
						res.RuleMetadata.Severity = severity
						results = append(results, res)
					}
				}
				return true
			})
		}
	}
	return results
}
