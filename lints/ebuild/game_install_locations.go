package ebuild

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleGameInstallLocations = lints.RuleMetadata{
	ID:          "GameInstallLocations",
	Title:       "Game Install Locations",
	Description: "The historical game install locations (/usr/games and /etc/games) must not be used anymore.",
	URL:         "https://projects.gentoo.org/qa/policy-guide/filesystem.html#pg0205",
	Severity:    lints.SeverityError,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "filesystem", "PG0205"},
}

func init() {
	lints.RegisterRuleMetadata(ruleGameInstallLocations)
	lints.RegisterLintRule(&GameInstallLocationsLintRule{})
}

type GameInstallLocationsLintRule struct{}

func isGameInstallLocation(word *syntax.Word) (bool, string) {
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
				}
			}
		}
	}

	s := fullString.String()
	sClean := strings.Trim(s, "\"'")
	sClean = filepath.Clean(sClean)

	if strings.HasPrefix(sClean, "/usr/games") || strings.HasPrefix(sClean, "/etc/games") {
		return true, sClean
	}
	return false, ""
}

func (l *GameInstallLocationsLintRule) Lint(repoDir string, pkgData *g2.PackageData) []lints.LintResult {
	return l.LintWithQA(repoDir, pkgData, nil)
}

func (l *GameInstallLocationsLintRule) LintWithQA(repoDir string, pkgData *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult

	severity := lints.SeverityError
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0205"]; ok {
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

				if cmdName == "into" || cmdName == "insinto" || cmdName == "exeinto" || cmdName == "docinto" || cmdName == "dodir" || cmdName == "keepdir" || cmdName == "diropts" || cmdName == "exeopts" || cmdName == "insopts" || cmdName == "libopts" {
					for i := 1; i < len(cmd.Args); i++ {
						isGameInstall, val := isGameInstallLocation(cmd.Args[i])
						if isGameInstall {
							res := lints.LintResult{
								RuleMetadata: ruleGameInstallLocations,
								Message:      fmt.Sprintf("[%s] Ebuild %s attempts to install into historical game location using '%s %s', which must not be used anymore.", severity, version.Ebuild.Path, cmdName, val),
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
