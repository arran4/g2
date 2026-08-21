package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleUsrLocal = lints.RuleMetadata{
	ID:          "UsrLocal",
	Title:       "Installation to /usr/local",
	Description: "Ebuilds must not install into /usr/local as it is reserved for non-Portage applications.",
	References: []lints.RuleReference{
		{URL: "https://devmanual.gentoo.org/general-concepts/filesystem/index.html", Label: "Gentoo Devmanual"},
	},
	Severity: lints.SeverityError,
	Source:   lints.SourceG2,
	Tags:     []string{"ebuild", "gentoo-policy", "filesystem"},
}

func init() {
	lints.RegisterRuleMetadata(ruleUsrLocal)
	lints.RegisterLintRule(&UsrLocalLintRule{})
}

type UsrLocalLintRule struct{}

func isUsrLocalPath(word *syntax.Word) (bool, string) {
	var fullString strings.Builder
	for _, part := range word.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			fullString.WriteString(p.Value)
		case *syntax.SglQuoted:
			fullString.WriteString(p.Value)
		case *syntax.DblQuoted:
			for _, pp := range p.Parts {
				switch ppp := pp.(type) {
				case *syntax.Lit:
					fullString.WriteString(ppp.Value)
				case *syntax.ParamExp:
					// For parameter expansion like ${EPREFIX}, we can just ignore it for the prefix check
					// e.g. "${EPREFIX}/usr/local/bin" -> "/usr/local/bin"
				}
			}
		}
	}

	s := fullString.String()
	if strings.HasPrefix(s, "/usr/local") || strings.HasPrefix(s, "\"/usr/local") || strings.HasPrefix(s, "'/usr/local") {
		return true, s
	}
	return false, ""
}

func (l *UsrLocalLintRule) Lint(repoDir string, pkgData *g2.PackageData) []lints.LintResult {
	var results []lints.LintResult

	for _, version := range pkgData.Versions {
		if version.Ebuild == nil || version.Ebuild.RawText == "" {
			continue
		}

		parser := syntax.NewParser()
		f, err := parser.Parse(strings.NewReader(version.Ebuild.RawText), "")
		if err != nil {
			continue
		}

		syntax.Walk(f, func(node syntax.Node) bool {
			cmd, ok := node.(*syntax.CallExpr)
			if !ok {
				return true
			}

			if len(cmd.Args) > 1 {
				var cmdName string
				if len(cmd.Args[0].Parts) == 1 {
					if lit, ok := cmd.Args[0].Parts[0].(*syntax.Lit); ok {
						cmdName = lit.Value
					}
				}

				if cmdName == "into" || cmdName == "insinto" || cmdName == "exeinto" || cmdName == "docinto" || cmdName == "dodir" || cmdName == "keepdir" || cmdName == "diropts" || cmdName == "exeopts" || cmdName == "insopts" || cmdName == "libopts" {
					for i := 1; i < len(cmd.Args); i++ {
						isUsrLocal, val := isUsrLocalPath(cmd.Args[i])
						if isUsrLocal {
							res := lints.LintResult{
								RuleMetadata: ruleUsrLocal,
								Message:      fmt.Sprintf("[Error] Ebuild %s attempts to install into /usr/local using '%s %s', which is reserved for non-Portage applications.", version.Ebuild.Path, cmdName, val),
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
