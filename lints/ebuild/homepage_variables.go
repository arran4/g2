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

var ruleHomepageVariables = lints.RuleMetadata{
	ID:          "HomepageVariables",
	Title:       "HOMEPAGE contains variables",
	Description: "HOMEPAGE must specify all URIs verbatim, without referring to variables.",
	URL:         "https://projects.gentoo.org/qa/policy-guide/ebuild-format.html#pg0103",
	Severity:    lints.SeverityError,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "PG0103"},
}

func init() {
	lints.RegisterRuleMetadata(ruleHomepageVariables)
	lints.RegisterLintRule(&HomepageVariablesLintRule{})
}

type HomepageVariablesLintRule struct{}

func (r *HomepageVariablesLintRule) Lint(repoDir string, pkg *g2.PackageData, ctx *lints.LintContext) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil, ctx)
}

func (r *HomepageVariablesLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy, ctx *lints.LintContext) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError

	if qa != nil {
		if val, ok := qa.Policies["PG0103"]; ok {
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
				if assign, ok := node.(*syntax.Assign); ok {
					if assign.Name != nil && assign.Name.Value == "HOMEPAGE" {
						syntax.Walk(assign.Value, func(inner syntax.Node) bool {
							switch nx := inner.(type) {
							case *syntax.ParamExp:
								res := lints.LintResult{
									RuleMetadata: ruleHomepageVariables,
									Message:      fmt.Sprintf("[%s] Ebuild %s HOMEPAGE must not contain variables (found '%s')", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version, nx.Param.Value),
									Package:      pkg.Category + "/" + pkg.Name,
								}
								res.RuleMetadata.Severity = severity
								results = append(results, res)
							}
							return true
						})
					}
				}
				return true
			})
		}
	}
	return results
}
