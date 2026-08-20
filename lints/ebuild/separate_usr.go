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

var ruleSeparateUsr = lints.RuleMetadata{
	ID:          "SeparateUsr",
	Title:       "Support for separate /usr",
	Description: "Developers are not required to support using separate /usr filesystem without an initramfs. Warnings regarding this are typically suppressed or not fatal.",
	URL:         "https://projects.gentoo.org/qa/policy-guide/filesystem.html#pg0202",
	Severity:    lints.SeverityInfo,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "filesystem", "PG0202"},
}

func init() {
	lints.RegisterRuleMetadata(ruleSeparateUsr)
	lints.RegisterLintRule(&SeparateUsrLintRule{})
}

type SeparateUsrLintRule struct{}

func (l *SeparateUsrLintRule) Lint(repoDir string, pkgData *g2.PackageData) []lints.LintResult {
	return l.LintWithQA(repoDir, pkgData, nil)
}

func (l *SeparateUsrLintRule) LintWithQA(repoDir string, pkgData *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := ruleSeparateUsr.Severity

	if qa != nil {
		if val, ok := qa.Policies["PG0202"]; ok {
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

	_ = severity // The rule is currently mostly informational (PG0202 says it's NOT required to support separate /usr).

	for _, version := range pkgData.Versions {
		if version.Ebuild == nil || version.Ebuild.RawText == "" {
			continue
		}

		parser := syntax.NewParser(syntax.KeepComments(true))
		f, err := parser.Parse(strings.NewReader(version.Ebuild.RawText), "")
		if err != nil {
			continue
		}

		syntax.Walk(f, func(node syntax.Node) bool {
			switch nx := node.(type) {
			case *syntax.CallExpr:
				if len(nx.Args) > 0 {
					if len(nx.Args[0].Parts) == 0 {
						return true
					}
					lit, ok := nx.Args[0].Parts[0].(*syntax.Lit)
					if !ok {
						return true
					}
					cmd := lit.Value
					if cmd == "insinto" || cmd == "exeinto" || cmd == "dodir" || cmd == "into" {
						if len(nx.Args) > 1 {
							if len(nx.Args[1].Parts) == 0 {
								return true
							}
							pathLit, ok := nx.Args[1].Parts[0].(*syntax.Lit)
							if !ok {
								return true
							}
							path := pathLit.Value

							// PG0202 refers to separate /usr policy.
							// Usually this warns on dodir /bin, /sbin, /lib, /lib64.
							if path == "/bin" || path == "/sbin" || path == "/lib" || path == "/lib64" {
								res := lints.LintResult{
									RuleMetadata: ruleSeparateUsr,
									Message:      fmt.Sprintf("[%s] Ebuild %s attempts to install into %s using %s, which may violate separate /usr policy (PG0202).", cases.Title(language.Und, cases.NoLower).String(string(severity)), version.Version, path, cmd),
									Package:      pkgData.Category + "/" + pkgData.Name,
								}
								res.RuleMetadata.Severity = severity
								results = append(results, res)
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
