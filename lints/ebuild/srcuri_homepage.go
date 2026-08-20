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

var ruleSrcUriHomepage = lints.RuleMetadata{
	ID:          "SrcUriHomepage",
	Title:       "SRC_URI refers to HOMEPAGE",
	Description: "SRC_URI must not refer to ${HOMEPAGE}.",
	URLs:        []string{"https://projects.gentoo.org/qa/policy-guide/ebuild-format.html#pg0104"},
	Severity:    lints.SeverityError,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "PG0104"},
}

func init() {
	lints.RegisterRuleMetadata(ruleSrcUriHomepage)
	lints.RegisterLintRule(&SrcUriHomepageLintRule{})
}

type SrcUriHomepageLintRule struct{}

func (r *SrcUriHomepageLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *SrcUriHomepageLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError

	if qa != nil {
		if val, ok := qa.Policies["PG0104"]; ok {
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
					if assign.Name != nil && assign.Name.Value == "SRC_URI" {
						syntax.Walk(assign.Value, func(inner syntax.Node) bool {
							switch nx := inner.(type) {
							case *syntax.ParamExp:
								if nx.Param.Value == "HOMEPAGE" {
									res := lints.LintResult{
										RuleMetadata: ruleSrcUriHomepage,
										Message:      fmt.Sprintf("[%s] Ebuild %s SRC_URI must not refer to ${HOMEPAGE}", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
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
				return true
			})
		}
	}
	return results
}
