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

var ruleKeywordsSingleLine = lints.RuleMetadata{
	ID:          "KeywordsSingleLine",
	Title:       "KEYWORDS defined on a single line",
	Description: "KEYWORDS must be defined at most once in an ebuild, on a single line, with literal content.",
	URLs:        []string{"https://projects.gentoo.org/qa/policy-guide/ebuild-format.html#pg0105"},
	Severity:    lints.SeverityError,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "PG0105"},
}

func init() {
	lints.RegisterRuleMetadata(ruleKeywordsSingleLine)
	lints.RegisterLintRule(&KeywordsSingleLineLintRule{})
}

type KeywordsSingleLineLintRule struct{}

func (r *KeywordsSingleLineLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *KeywordsSingleLineLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError

	if qa != nil {
		if val, ok := qa.Policies["PG0105"]; ok {
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

			keywordAssignCount := 0
			syntax.Walk(f, func(node syntax.Node) bool {
				if assign, ok := node.(*syntax.Assign); ok {
					if assign.Name != nil && assign.Name.Value == "KEYWORDS" {
						keywordAssignCount++

						// Check append
						if assign.Append {
							res := lints.LintResult{
								RuleMetadata: ruleKeywordsSingleLine,
								Message:      fmt.Sprintf("[%s] Ebuild %s appends to KEYWORDS. It must be defined at most once.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
								Package:      pkg.Category + "/" + pkg.Name,
							}
							res.RuleMetadata.Severity = severity
							results = append(results, res)
						}

						// Check literal content and single line
						hasVarRef := false
						hasNewlines := false

						syntax.Walk(assign.Value, func(inner syntax.Node) bool {
							switch nx := inner.(type) {
							case *syntax.ParamExp:
								hasVarRef = true
							case *syntax.Lit:
								if strings.Contains(nx.Value, "\n") {
									hasNewlines = true
								}
							}
							return true
						})

						if hasVarRef {
							res := lints.LintResult{
								RuleMetadata: ruleKeywordsSingleLine,
								Message:      fmt.Sprintf("[%s] Ebuild %s KEYWORDS contains variable references. It must have literal content.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
								Package:      pkg.Category + "/" + pkg.Name,
							}
							res.RuleMetadata.Severity = severity
							results = append(results, res)
						}
						if hasNewlines {
							res := lints.LintResult{
								RuleMetadata: ruleKeywordsSingleLine,
								Message:      fmt.Sprintf("[%s] Ebuild %s KEYWORDS contains newlines. It must be defined on a single line.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
								Package:      pkg.Category + "/" + pkg.Name,
							}
							res.RuleMetadata.Severity = severity
							results = append(results, res)
						}
					}
				}
				return true
			})

			if keywordAssignCount > 1 {
				res := lints.LintResult{
					RuleMetadata: ruleKeywordsSingleLine,
					Message:      fmt.Sprintf("[%s] Ebuild %s KEYWORDS is defined multiple times. It must be defined at most once.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}
		}
	}
	return results
}
