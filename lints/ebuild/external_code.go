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

var ruleExternalCode = lints.RuleMetadata{
	ID:          "ExternalCode",
	Title:       "Code must be contained within ebuild and eclasses",
	Description: "It is forbidden to load additional ebuild code from other files via source, eval or any other possible method.",
	References: []lints.RuleReference{
		{URL: "https://projects.gentoo.org/qa/policy-guide/ebuild-format.html#pg0102", Label: "Gentoo QA Policy Guide PG0102"},
	},
	Severity: lints.SeverityError,
	Source:   lints.SourceQA,
	Tags:     []string{"ebuild", "gentoo-policy", "PG0102"},
}

func init() {
	lints.RegisterRuleMetadata(ruleExternalCode)
	lints.RegisterLintRule(&ExternalCodeLintRule{})
}

type ExternalCodeLintRule struct{}

func (r *ExternalCodeLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *ExternalCodeLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError

	if qa != nil {
		if val, ok := qa.Policies["PG0102"]; ok {
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
				case *syntax.CallExpr:
					if len(n.Args) > 0 && len(n.Args[0].Parts) > 0 {
						if lit, ok := n.Args[0].Parts[0].(*syntax.Lit); ok {
							cmd := lit.Value
							if cmd == "source" || cmd == "." || cmd == "eval" {
								// We should probably check if it's sourcing something other than eclasses if possible?
								// PG0102 just says "It is forbidden to load additional ebuild code from other files via source, eval or any other possible method."
								// eclasses are usually inherited, not sourced manually in ebuilds (except inside inherit itself which isn't an ebuild).
								// Let's just flag source/. and eval
								res := lints.LintResult{
									RuleMetadata: ruleExternalCode,
									Message:      fmt.Sprintf("[%s] Ebuild %s sources external code using '%s'. Code must be contained within ebuild and eclasses.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version, cmd),
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
