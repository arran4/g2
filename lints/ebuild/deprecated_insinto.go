package ebuild

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleDeprecatedInsinto = lints.RuleMetadata{
	ID:          "DeprecatedInsinto",
	Title:       "Deprecated insinto/exeinto Usage",
	Description: "Detects the use of insinto or exeinto for paths that have dedicated install functions (e.g. doinitd for /etc/init.d).",
	URL:         "https://devmanual.gentoo.org/tasks-reference/init-scripts/index.html",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceG2,
	Tags:        []string{"ebuild", "gentoo-policy", "PG0805"},
}

func init() {
	lints.RegisterRuleMetadata(ruleDeprecatedInsinto)
	lints.RegisterLintRule(&DeprecatedInsintoLintRule{})
}

type DeprecatedInsintoLintRule struct{}

func isDeprecatedInsintoPath(word *syntax.Word) (bool, string, string) {
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
					// Ignore parameter expansion
				}
			}
		}
	}

	s := fullString.String()
	sClean := strings.Trim(s, "\"'") // In case quotes were literal but not captured by parser correctly
	sClean = filepath.Clean(sClean)

	pathMapping := map[string]string{
		"/etc/conf.d":                          "doconfd or newconfd",
		"/etc/env.d":                           "doenvd or newenvd",
		"/etc/init.d":                          "doinitd or newinitd",
		"/etc/pam.d":                           "dopamd or newpamd from pam.eclass",
		"/usr/lib/systemd/system":              "systemd_dounit or systemd_newunit from systemd.eclass",
		"/usr/lib/systemd/user":                "systemd_douserunit or systemd_newuserunit from systemd.eclass",
		"/usr/share/applications":              "domenu or newmenu from desktop.eclass",
		"/usr/share/fish/vendor_completions.d": "dofishcomp or newfishcomp from shell-completion.eclass",
		"/usr/share/zsh/site-functions":        "dozshcomp or newzshcomp from shell-completion.eclass",
	}

	for prefix, suggestion := range pathMapping {
		if sClean == prefix || strings.HasPrefix(sClean, prefix+"/") {
			return true, s, suggestion
		}
	}
	return false, "", ""
}

func (l *DeprecatedInsintoLintRule) Lint(repoDir string, pkgData *g2.PackageData) []lints.LintResult {
	return l.LintWithQA(repoDir, pkgData, nil)
}

func (l *DeprecatedInsintoLintRule) LintWithQA(repoDir string, pkgData *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult

	severity := lints.SeverityWarning
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0805"]; ok {
			if val == "ignore" {
				return nil
			}
			switch val {
			case "notice":
				severity = lints.SeverityNotice
			case "error":
				severity = lints.SeverityError
			case "warning":
				severity = lints.SeverityWarning
			}
		}
	}

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

				if cmdName == "insinto" || cmdName == "exeinto" {
					for i := 1; i < len(cmd.Args); i++ {
						isDeprecated, val, suggestion := isDeprecatedInsintoPath(cmd.Args[i])
						if isDeprecated {
							res := lints.LintResult{
								RuleMetadata: ruleDeprecatedInsinto,
								Message:      fmt.Sprintf("[%s] Ebuild %s uses '%s %s' which is deprecated. Use %s instead.", severity, version.Ebuild.Path, cmdName, val, suggestion),
								Package:      pkgData.Category + "/" + pkgData.Name,
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

	return results
}
